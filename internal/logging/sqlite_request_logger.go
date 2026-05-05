package logging

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usagestore"
	log "github.com/sirupsen/logrus"
)

type SQLiteRequestLogger struct {
	enabled atomic.Bool
	store   *usagestore.Store

	mu      sync.Mutex
	closed  bool
	queue   chan sqliteLogTask
	done    chan struct{}
	close   sync.Once
	dropped atomic.Uint64
}

type sqliteLogTask struct {
	args  asyncLogArgs
	force bool
}

func NewSQLiteRequestLogger(enabled bool, store *usagestore.Store) *SQLiteRequestLogger {
	l := &SQLiteRequestLogger{
		store: store,
		queue: make(chan sqliteLogTask, asyncNormalQueueDepth),
		done:  make(chan struct{}),
	}
	l.enabled.Store(enabled)
	go l.run()
	return l
}

func (l *SQLiteRequestLogger) IsEnabled() bool {
	if l == nil {
		return false
	}
	return l.enabled.Load()
}

func (l *SQLiteRequestLogger) SetEnabled(enabled bool) {
	if l != nil {
		l.enabled.Store(enabled)
	}
}

func (l *SQLiteRequestLogger) SetErrorLogsMaxFiles(_ int) {}

func (l *SQLiteRequestLogger) DroppedLogs() uint64 {
	if l == nil {
		return 0
	}
	return l.dropped.Load()
}

func (l *SQLiteRequestLogger) Close() {
	if l == nil {
		return
	}
	l.close.Do(func() {
		l.mu.Lock()
		l.closed = true
		close(l.queue)
		l.mu.Unlock()
		<-l.done
	})
}

func (l *SQLiteRequestLogger) LogRequest(url, method string, requestHeaders map[string][]string, body []byte, statusCode int, responseHeaders map[string][]string, response, websocketTimeline, apiRequest, apiResponse, apiWebsocketTimeline []byte, apiResponseErrors []*interfaces.ErrorMessage, requestID string, requestTimestamp, apiResponseTimestamp time.Time) error {
	return l.LogRequestWithOptions(url, method, requestHeaders, body, statusCode, responseHeaders, response, websocketTimeline, apiRequest, apiResponse, apiWebsocketTimeline, apiResponseErrors, false, requestID, requestTimestamp, apiResponseTimestamp)
}

func (l *SQLiteRequestLogger) LogRequestWithOptions(url, method string, requestHeaders map[string][]string, body []byte, statusCode int, responseHeaders map[string][]string, response, websocketTimeline, apiRequest, apiResponse, apiWebsocketTimeline []byte, apiResponseErrors []*interfaces.ErrorMessage, force bool, requestID string, requestTimestamp, apiResponseTimestamp time.Time) error {
	if l == nil || l.store == nil {
		return nil
	}
	if !l.enabled.Load() && !force {
		return nil
	}
	args := cloneArgsForAsync(asyncLogArgs{
		url:                  url,
		method:               method,
		requestHeaders:       requestHeaders,
		body:                 body,
		statusCode:           statusCode,
		responseHeaders:      responseHeaders,
		response:             response,
		websocketTimeline:    websocketTimeline,
		apiRequest:           apiRequest,
		apiResponse:          apiResponse,
		apiWebsocketTimeline: apiWebsocketTimeline,
		apiResponseErrors:    apiResponseErrors,
		requestID:            requestID,
		requestTimestamp:     requestTimestamp,
		apiResponseTimestamp: apiResponseTimestamp,
	})
	return l.enqueue(sqliteLogTask{args: args, force: force})
}

func (l *SQLiteRequestLogger) LogStreamingRequest(url, method string, headers map[string][]string, body []byte, requestID string) (StreamingLogWriter, error) {
	if l == nil || l.store == nil || !l.enabled.Load() {
		return &NoOpStreamingLogWriter{}, nil
	}
	return &SQLiteStreamingLogWriter{
		logger:         l,
		url:            url,
		method:         method,
		requestHeaders: cloneHeader(headers),
		requestBody:    cloneBytes(body),
		requestID:      requestID,
		timestamp:      time.Now(),
	}, nil
}

func (l *SQLiteRequestLogger) enqueue(task sqliteLogTask) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		if task.force {
			return l.writeLogRequest(task.args, task.force)
		}
		return nil
	}
	select {
	case l.queue <- task:
		return nil
	default:
		if task.force {
			return l.writeLogRequest(task.args, task.force)
		}
		l.dropped.Add(1)
		return nil
	}
}

func (l *SQLiteRequestLogger) run() {
	defer close(l.done)
	for task := range l.queue {
		if err := l.writeLogRequest(task.args, task.force); err != nil {
			log.WithError(err).Warn("failed to persist request history")
		}
	}
}

func (l *SQLiteRequestLogger) writeLogRequest(args asyncLogArgs, force bool) error {
	if l == nil || l.store == nil {
		return nil
	}
	if args.requestID == "" {
		args.requestID = GenerateRequestID()
	}
	if args.requestTimestamp.IsZero() {
		args.requestTimestamp = time.Now()
	}

	helper := &FileRequestLogger{}
	responseToWrite, decompressErr := helper.decompressResponse(args.responseHeaders, args.response)
	var buf bytes.Buffer
	if err := helper.writeNonStreamingLog(
		&buf,
		args.url,
		args.method,
		args.requestHeaders,
		args.body,
		"",
		args.websocketTimeline,
		args.apiRequest,
		args.apiResponse,
		args.apiWebsocketTimeline,
		args.apiResponseErrors,
		args.statusCode,
		args.responseHeaders,
		responseToWrite,
		decompressErr,
		args.requestTimestamp,
		args.apiResponseTimestamp,
	); err != nil {
		return fmt.Errorf("format request history: %w", err)
	}

	logName := helper.generateFilename(args.url, args.requestID)
	if force {
		logName = "error-" + logName
	}
	return l.store.InsertRequestHistory(context.Background(), usagestore.RequestHistory{
		RequestID:            args.requestID,
		LogName:              logName,
		URL:                  args.url,
		Method:               args.method,
		StatusCode:           args.statusCode,
		Force:                force,
		RequestTimestamp:     args.requestTimestamp,
		APIResponseTimestamp: args.apiResponseTimestamp,
		LogText:              buf.Bytes(),
	})
}

type SQLiteStreamingLogWriter struct {
	logger *SQLiteRequestLogger

	url            string
	method         string
	requestHeaders map[string][]string
	requestBody    []byte
	requestID      string
	timestamp      time.Time

	mu                   sync.Mutex
	response             bytes.Buffer
	responseStatus       int
	statusWritten        bool
	responseHeaders      map[string][]string
	apiRequest           []byte
	apiResponse          []byte
	apiWebsocketTimeline []byte
	firstChunkTimestamp  time.Time
	apiResponseTimestamp time.Time
}

func (w *SQLiteStreamingLogWriter) WriteChunkAsync(chunk []byte) {
	if w == nil || len(chunk) == 0 {
		return
	}
	w.mu.Lock()
	_, _ = w.response.Write(chunk)
	w.mu.Unlock()
}

func (w *SQLiteStreamingLogWriter) WriteStatus(status int, headers map[string][]string) error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.responseStatus = status
	w.statusWritten = true
	w.responseHeaders = cloneHeader(headers)
	return nil
}

func (w *SQLiteStreamingLogWriter) WriteAPIRequest(apiRequest []byte) error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.apiRequest = cloneBytes(apiRequest)
	return nil
}

func (w *SQLiteStreamingLogWriter) WriteAPIResponse(apiResponse []byte) error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.apiResponse = cloneBytes(apiResponse)
	w.apiResponseTimestamp = time.Now()
	return nil
}

func (w *SQLiteStreamingLogWriter) WriteAPIWebsocketTimeline(apiWebsocketTimeline []byte) error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.apiWebsocketTimeline = cloneBytes(apiWebsocketTimeline)
	return nil
}

func (w *SQLiteStreamingLogWriter) SetFirstChunkTimestamp(timestamp time.Time) {
	if w == nil {
		return
	}
	w.mu.Lock()
	w.firstChunkTimestamp = timestamp
	w.mu.Unlock()
}

func (w *SQLiteStreamingLogWriter) Close() error {
	if w == nil || w.logger == nil {
		return nil
	}
	w.mu.Lock()
	args := asyncLogArgs{
		url:                  w.url,
		method:               w.method,
		requestHeaders:       cloneHeader(w.requestHeaders),
		body:                 cloneBytes(w.requestBody),
		statusCode:           w.responseStatus,
		responseHeaders:      cloneHeader(w.responseHeaders),
		response:             cloneBytes(w.response.Bytes()),
		apiRequest:           cloneBytes(w.apiRequest),
		apiResponse:          cloneBytes(w.apiResponse),
		apiWebsocketTimeline: cloneBytes(w.apiWebsocketTimeline),
		requestID:            w.requestID,
		requestTimestamp:     w.timestamp,
		apiResponseTimestamp: w.apiResponseTimestamp,
	}
	w.mu.Unlock()
	return w.logger.enqueue(sqliteLogTask{args: args})
}
