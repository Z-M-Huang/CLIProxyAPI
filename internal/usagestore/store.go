package usagestore

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
	_ "modernc.org/sqlite"
)

const (
	DefaultDatabasePath = "./data/usage.sqlite"
	migrationTableName  = "usage_schema_migrations"
)

type Store struct {
	db *sql.DB
}

type TokenStats struct {
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	ReasoningTokens     int64 `json:"reasoning_tokens"`
	CachedTokens        int64 `json:"cached_tokens"`
	CacheReadTokens     int64 `json:"cache_read_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_tokens"`
	TotalTokens         int64 `json:"total_tokens"`
}

type UsageEvent struct {
	EventKey    string
	APIGroupKey string
	Provider    string
	Endpoint    string
	AuthType    string
	RequestID   string
	Model       string
	Timestamp   time.Time
	Source      string
	AuthIndex   string
	Failed      bool
	LatencyMS   int64
	Tokens      TokenStats
}

type RequestDetail struct {
	Timestamp time.Time  `json:"timestamp"`
	LatencyMS int64      `json:"latency_ms"`
	Source    string     `json:"source"`
	AuthIndex string     `json:"auth_index"`
	Tokens    TokenStats `json:"tokens"`
	Failed    bool       `json:"failed"`
	RequestID string     `json:"request_id,omitempty"`
}

type StatisticsSnapshot struct {
	TotalRequests int64 `json:"total_requests"`
	SuccessCount  int64 `json:"success_count"`
	FailureCount  int64 `json:"failure_count"`
	TotalTokens   int64 `json:"total_tokens"`

	APIs map[string]APISnapshot `json:"apis"`

	RequestsByDay  map[string]int64 `json:"requests_by_day"`
	RequestsByHour map[string]int64 `json:"requests_by_hour"`
	TokensByDay    map[string]int64 `json:"tokens_by_day"`
	TokensByHour   map[string]int64 `json:"tokens_by_hour"`
}

type APISnapshot struct {
	TotalRequests int64                    `json:"total_requests"`
	TotalTokens   int64                    `json:"total_tokens"`
	Models        map[string]ModelSnapshot `json:"models"`
}

type ModelSnapshot struct {
	TotalRequests int64           `json:"total_requests"`
	TotalTokens   int64           `json:"total_tokens"`
	Details       []RequestDetail `json:"details"`
}

type MergeResult struct {
	Added   int64 `json:"added"`
	Skipped int64 `json:"skipped"`
}

type UsageEventRecord struct {
	ID          int64      `json:"id"`
	Timestamp   time.Time  `json:"timestamp"`
	APIGroupKey string     `json:"api_group_key"`
	Provider    string     `json:"provider"`
	Endpoint    string     `json:"endpoint"`
	AuthType    string     `json:"auth_type"`
	RequestID   string     `json:"request_id"`
	Model       string     `json:"model"`
	Source      string     `json:"source"`
	AuthIndex   string     `json:"auth_index"`
	Failed      bool       `json:"failed"`
	LatencyMS   int64      `json:"latency_ms"`
	Tokens      TokenStats `json:"tokens"`
}

type UsageEventsPage struct {
	Events     []UsageEventRecord `json:"events"`
	Models     []string           `json:"models"`
	Sources    []string           `json:"sources"`
	TotalCount int64              `json:"total_count"`
	Page       int                `json:"page"`
	PageSize   int                `json:"page_size"`
	TotalPages int                `json:"total_pages"`
}

type UsageEventsFilter struct {
	StartTime *time.Time
	EndTime   *time.Time
	Page      int
	PageSize  int
	Model     string
	Source    string
	Result    string
}

type RequestHistory struct {
	RequestID            string
	LogName              string
	URL                  string
	Method               string
	StatusCode           int
	Force                bool
	RequestTimestamp     time.Time
	APIResponseTimestamp time.Time
	LogText              []byte
}

type RequestHistoryFile struct {
	Name     string `json:"name"`
	Size     int64  `json:"size,omitempty"`
	Modified int64  `json:"modified,omitempty"`
}

func Open(path string) (*Store, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = DefaultDatabasePath
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("usage store: create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("usage store: open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &Store{db: db}
	if err := store.configure(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) configure() error {
	for _, statement := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("usage store: %s: %w", statement, err)
		}
	}
	return nil
}

func (s *Store) migrate() error {
	migrationDir, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("usage store: load migrations: %w", err)
	}
	store, err := database.NewStore(database.DialectSQLite3, migrationTableName)
	if err != nil {
		return fmt.Errorf("usage store: create migration store: %w", err)
	}
	provider, err := goose.NewProvider("", s.db, migrationDir, goose.WithStore(store), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		return fmt.Errorf("usage store: create migration provider: %w", err)
	}
	if _, err = provider.Up(context.Background()); err != nil {
		return fmt.Errorf("usage store: migrate: %w", err)
	}
	return nil
}

func (s *Store) InsertUsageEvent(ctx context.Context, event UsageEvent) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("usage store is nil")
	}
	event = NormalizeUsageEvent(event)
	result, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO usage_events (
		event_key, api_group_key, provider, endpoint, auth_type, request_id, model, timestamp,
		source, auth_index, failed, latency_ms, input_tokens, output_tokens, reasoning_tokens,
		cached_tokens, cache_read_tokens, cache_creation_tokens, total_tokens, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.EventKey,
		event.APIGroupKey,
		event.Provider,
		event.Endpoint,
		event.AuthType,
		event.RequestID,
		event.Model,
		formatTime(event.Timestamp),
		event.Source,
		event.AuthIndex,
		boolInt(event.Failed),
		event.LatencyMS,
		event.Tokens.InputTokens,
		event.Tokens.OutputTokens,
		event.Tokens.ReasoningTokens,
		event.Tokens.CachedTokens,
		event.Tokens.CacheReadTokens,
		event.Tokens.CacheCreationTokens,
		event.Tokens.TotalTokens,
		formatTime(time.Now().UTC()),
	)
	if err != nil {
		return false, fmt.Errorf("usage store: insert usage event: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("usage store: usage event rows affected: %w", err)
	}
	return rows > 0, nil
}

func (s *Store) ImportSnapshot(ctx context.Context, snapshot StatisticsSnapshot) (MergeResult, error) {
	result := MergeResult{}
	if snapshot.APIs == nil {
		return result, nil
	}
	for apiName, apiSnapshot := range snapshot.APIs {
		for modelName, modelSnapshot := range apiSnapshot.Models {
			for _, detail := range modelSnapshot.Details {
				inserted, err := s.InsertUsageEvent(ctx, UsageEvent{
					APIGroupKey: strings.TrimSpace(apiName),
					Model:       strings.TrimSpace(modelName),
					Timestamp:   detail.Timestamp,
					Source:      detail.Source,
					AuthIndex:   detail.AuthIndex,
					Failed:      detail.Failed,
					LatencyMS:   detail.LatencyMS,
					RequestID:   detail.RequestID,
					Tokens:      detail.Tokens,
				})
				if err != nil {
					return result, err
				}
				if inserted {
					result.Added++
				} else {
					result.Skipped++
				}
			}
		}
	}
	return result, nil
}

func (s *Store) BuildSnapshot(ctx context.Context) (StatisticsSnapshot, error) {
	snapshot := newStatisticsSnapshot()
	if s == nil || s.db == nil {
		return snapshot, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, event_key, api_group_key, provider, endpoint, auth_type, request_id,
		model, timestamp, source, auth_index, failed, latency_ms, input_tokens, output_tokens,
		reasoning_tokens, cached_tokens, cache_read_tokens, cache_creation_tokens, total_tokens
		FROM usage_events ORDER BY timestamp ASC, id ASC`)
	if err != nil {
		return snapshot, fmt.Errorf("usage store: query usage events: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		event, errScan := scanUsageEvent(rows)
		if errScan != nil {
			return snapshot, errScan
		}
		applyEventToSnapshot(&snapshot, event)
	}
	if err := rows.Err(); err != nil {
		return snapshot, fmt.Errorf("usage store: read usage events: %w", err)
	}
	return snapshot, nil
}

func (s *Store) ListUsageEvents(ctx context.Context, filter UsageEventsFilter) (UsageEventsPage, error) {
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageSize > 1000 {
		pageSize = 1000
	}

	where, args := usageEventsWhere(filter)
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_events"+where, args...).Scan(&total); err != nil {
		return UsageEventsPage{}, fmt.Errorf("usage store: count usage events: %w", err)
	}

	queryArgs := append([]any(nil), args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT id, event_key, api_group_key, provider, endpoint, auth_type, request_id,
		model, timestamp, source, auth_index, failed, latency_ms, input_tokens, output_tokens,
		reasoning_tokens, cached_tokens, cache_read_tokens, cache_creation_tokens, total_tokens
		FROM usage_events`+where+` ORDER BY timestamp DESC, id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return UsageEventsPage{}, fmt.Errorf("usage store: list usage events: %w", err)
	}
	defer rows.Close()

	events := make([]UsageEventRecord, 0, pageSize)
	for rows.Next() {
		event, errScan := scanUsageEvent(rows)
		if errScan != nil {
			return UsageEventsPage{}, errScan
		}
		events = append(events, UsageEventRecord{
			ID:          event.ID,
			Timestamp:   event.Timestamp,
			APIGroupKey: event.APIGroupKey,
			Provider:    event.Provider,
			Endpoint:    event.Endpoint,
			AuthType:    event.AuthType,
			RequestID:   event.RequestID,
			Model:       event.Model,
			Source:      event.Source,
			AuthIndex:   event.AuthIndex,
			Failed:      event.Failed,
			LatencyMS:   event.LatencyMS,
			Tokens:      event.Tokens,
		})
	}
	if err := rows.Err(); err != nil {
		return UsageEventsPage{}, fmt.Errorf("usage store: read usage events page: %w", err)
	}

	models, err := s.listDistinct(ctx, "model", filter)
	if err != nil {
		return UsageEventsPage{}, err
	}
	sources, err := s.listDistinct(ctx, "source", filter)
	if err != nil {
		return UsageEventsPage{}, err
	}
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return UsageEventsPage{
		Events:     events,
		Models:     models,
		Sources:    sources,
		TotalCount: total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *Store) InsertRequestHistory(ctx context.Context, history RequestHistory) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("usage store is nil")
	}
	history.RequestID = strings.TrimSpace(history.RequestID)
	if history.RequestID == "" {
		return fmt.Errorf("usage store: request history request_id is required")
	}
	history.LogName = strings.TrimSpace(history.LogName)
	if history.LogName == "" {
		history.LogName = "request-" + history.RequestID + ".log"
	}
	compressed, err := gzipBytes(history.LogText)
	if err != nil {
		return fmt.Errorf("usage store: compress request history: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO request_histories (
		request_id, log_name, url, method, status_code, force, request_timestamp,
		api_response_timestamp, log_text_gzip, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(request_id) DO UPDATE SET
		log_name = excluded.log_name,
		url = excluded.url,
		method = excluded.method,
		status_code = excluded.status_code,
		force = excluded.force,
		request_timestamp = excluded.request_timestamp,
		api_response_timestamp = excluded.api_response_timestamp,
		log_text_gzip = excluded.log_text_gzip`,
		history.RequestID,
		history.LogName,
		strings.TrimSpace(history.URL),
		strings.TrimSpace(history.Method),
		history.StatusCode,
		boolInt(history.Force),
		formatOptionalTime(history.RequestTimestamp),
		formatOptionalTime(history.APIResponseTimestamp),
		compressed,
		formatTime(time.Now().UTC()),
	)
	if err != nil {
		return fmt.Errorf("usage store: insert request history: %w", err)
	}
	return nil
}

func (s *Store) RequestHistoryByID(ctx context.Context, requestID string) (RequestHistory, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return RequestHistory{}, fmt.Errorf("request_id is required")
	}
	var history RequestHistory
	var force int
	var requestTimestamp, apiResponseTimestamp string
	var compressed []byte
	err := s.db.QueryRowContext(ctx, `SELECT request_id, log_name, url, method, status_code, force,
		request_timestamp, api_response_timestamp, log_text_gzip
		FROM request_histories WHERE request_id = ?`, requestID).Scan(
		&history.RequestID,
		&history.LogName,
		&history.URL,
		&history.Method,
		&history.StatusCode,
		&force,
		&requestTimestamp,
		&apiResponseTimestamp,
		&compressed,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RequestHistory{}, os.ErrNotExist
	}
	if err != nil {
		return RequestHistory{}, fmt.Errorf("usage store: load request history: %w", err)
	}
	history.Force = force != 0
	history.RequestTimestamp = parseTime(requestTimestamp)
	history.APIResponseTimestamp = parseTime(apiResponseTimestamp)
	logText, err := gunzipBytes(compressed)
	if err != nil {
		return RequestHistory{}, fmt.Errorf("usage store: decompress request history: %w", err)
	}
	history.LogText = logText
	return history, nil
}

func (s *Store) RequestHistoryByLogName(ctx context.Context, name string) (RequestHistory, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return RequestHistory{}, fmt.Errorf("log name is required")
	}
	var requestID string
	err := s.db.QueryRowContext(ctx, `SELECT request_id FROM request_histories WHERE log_name = ? AND force = 1`, name).Scan(&requestID)
	if errors.Is(err, sql.ErrNoRows) {
		return RequestHistory{}, os.ErrNotExist
	}
	if err != nil {
		return RequestHistory{}, fmt.Errorf("usage store: find request history by log name: %w", err)
	}
	return s.RequestHistoryByID(ctx, requestID)
}

func (s *Store) ListErrorRequestHistories(ctx context.Context) ([]RequestHistoryFile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT log_name, LENGTH(log_text_gzip), created_at
		FROM request_histories WHERE force = 1 ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("usage store: list error request histories: %w", err)
	}
	defer rows.Close()
	files := make([]RequestHistoryFile, 0)
	for rows.Next() {
		var file RequestHistoryFile
		var createdAt string
		if errScan := rows.Scan(&file.Name, &file.Size, &createdAt); errScan != nil {
			return nil, fmt.Errorf("usage store: scan error request history: %w", errScan)
		}
		file.Modified = parseTime(createdAt).Unix()
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("usage store: read error request histories: %w", err)
	}
	return files, nil
}

func NormalizeUsageEvent(event UsageEvent) UsageEvent {
	event.APIGroupKey = firstNonEmpty(event.APIGroupKey, event.Provider, event.Endpoint, "unknown")
	event.Provider = strings.TrimSpace(event.Provider)
	event.Endpoint = strings.TrimSpace(event.Endpoint)
	event.AuthType = normalizeAuthType(event.AuthType)
	event.RequestID = strings.TrimSpace(event.RequestID)
	event.Model = firstNonEmpty(event.Model, "unknown")
	event.Timestamp = event.Timestamp.UTC()
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	event.Source = strings.TrimSpace(event.Source)
	event.AuthIndex = strings.TrimSpace(event.AuthIndex)
	event.Tokens = NormalizeTokenStats(event.Tokens)
	if event.LatencyMS < 0 {
		event.LatencyMS = 0
	}
	event.EventKey = strings.TrimSpace(event.EventKey)
	if event.EventKey == "" {
		event.EventKey = BuildEventKey(event)
	}
	return event
}

func NormalizeTokenStats(tokens TokenStats) TokenStats {
	if tokens.TotalTokens == 0 {
		tokens.TotalTokens = tokens.InputTokens + tokens.OutputTokens + tokens.ReasoningTokens
	}
	if tokens.TotalTokens == 0 {
		tokens.TotalTokens = tokens.InputTokens + tokens.OutputTokens + tokens.ReasoningTokens + tokens.CachedTokens
	}
	return tokens
}

func BuildEventKey(event UsageEvent) string {
	tokens := NormalizeTokenStats(event.Tokens)
	payload := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%s|%s|%s|%t|%d|%d|%d|%d|%d|%d|%d",
		strings.TrimSpace(event.APIGroupKey),
		strings.TrimSpace(event.Provider),
		strings.TrimSpace(event.Endpoint),
		strings.TrimSpace(event.AuthType),
		strings.TrimSpace(event.RequestID),
		strings.TrimSpace(event.Model),
		event.Timestamp.UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(event.Source),
		strings.TrimSpace(event.AuthIndex),
		event.Failed,
		tokens.InputTokens,
		tokens.OutputTokens,
		tokens.ReasoningTokens,
		tokens.CachedTokens,
		tokens.CacheReadTokens,
		tokens.CacheCreationTokens,
		tokens.TotalTokens,
	)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

type scannedUsageEvent struct {
	ID          int64
	EventKey    string
	APIGroupKey string
	Provider    string
	Endpoint    string
	AuthType    string
	RequestID   string
	Model       string
	Timestamp   time.Time
	Source      string
	AuthIndex   string
	Failed      bool
	LatencyMS   int64
	Tokens      TokenStats
}

func scanUsageEvent(rows interface {
	Scan(dest ...any) error
}) (scannedUsageEvent, error) {
	var event scannedUsageEvent
	var timestamp string
	var failed int
	if err := rows.Scan(
		&event.ID,
		&event.EventKey,
		&event.APIGroupKey,
		&event.Provider,
		&event.Endpoint,
		&event.AuthType,
		&event.RequestID,
		&event.Model,
		&timestamp,
		&event.Source,
		&event.AuthIndex,
		&failed,
		&event.LatencyMS,
		&event.Tokens.InputTokens,
		&event.Tokens.OutputTokens,
		&event.Tokens.ReasoningTokens,
		&event.Tokens.CachedTokens,
		&event.Tokens.CacheReadTokens,
		&event.Tokens.CacheCreationTokens,
		&event.Tokens.TotalTokens,
	); err != nil {
		return scannedUsageEvent{}, fmt.Errorf("usage store: scan usage event: %w", err)
	}
	event.Timestamp = parseTime(timestamp)
	event.Failed = failed != 0
	event.Tokens = NormalizeTokenStats(event.Tokens)
	return event, nil
}

func newStatisticsSnapshot() StatisticsSnapshot {
	return StatisticsSnapshot{
		APIs:           map[string]APISnapshot{},
		RequestsByDay:  map[string]int64{},
		RequestsByHour: map[string]int64{},
		TokensByDay:    map[string]int64{},
		TokensByHour:   map[string]int64{},
	}
}

func applyEventToSnapshot(snapshot *StatisticsSnapshot, event scannedUsageEvent) {
	apiName := firstNonEmpty(event.APIGroupKey, event.Provider, event.Endpoint, "unknown")
	modelName := firstNonEmpty(event.Model, "unknown")
	detail := RequestDetail{
		Timestamp: event.Timestamp.UTC(),
		LatencyMS: event.LatencyMS,
		Source:    event.Source,
		AuthIndex: event.AuthIndex,
		Tokens:    event.Tokens,
		Failed:    event.Failed,
		RequestID: event.RequestID,
	}

	apiSnapshot := snapshot.APIs[apiName]
	if apiSnapshot.Models == nil {
		apiSnapshot.Models = map[string]ModelSnapshot{}
	}
	modelSnapshot := apiSnapshot.Models[modelName]
	modelSnapshot.TotalRequests++
	modelSnapshot.TotalTokens += event.Tokens.TotalTokens
	modelSnapshot.Details = append(modelSnapshot.Details, detail)
	apiSnapshot.TotalRequests++
	apiSnapshot.TotalTokens += event.Tokens.TotalTokens
	apiSnapshot.Models[modelName] = modelSnapshot
	snapshot.APIs[apiName] = apiSnapshot

	snapshot.TotalRequests++
	if event.Failed {
		snapshot.FailureCount++
	} else {
		snapshot.SuccessCount++
	}
	snapshot.TotalTokens += event.Tokens.TotalTokens

	dayKey := event.Timestamp.Format("2006-01-02")
	hourKey := event.Timestamp.Format("15")
	snapshot.RequestsByDay[dayKey]++
	snapshot.RequestsByHour[hourKey]++
	snapshot.TokensByDay[dayKey] += event.Tokens.TotalTokens
	snapshot.TokensByHour[hourKey] += event.Tokens.TotalTokens
}

func usageEventsWhere(filter UsageEventsFilter) (string, []any) {
	clauses := make([]string, 0)
	args := make([]any, 0)
	if filter.StartTime != nil {
		clauses = append(clauses, "timestamp >= ?")
		args = append(args, formatTime(filter.StartTime.UTC()))
	}
	if filter.EndTime != nil {
		clauses = append(clauses, "timestamp <= ?")
		args = append(args, formatTime(filter.EndTime.UTC()))
	}
	if model := strings.TrimSpace(filter.Model); model != "" {
		clauses = append(clauses, "TRIM(model) = ?")
		args = append(args, model)
	}
	if source := strings.TrimSpace(filter.Source); source != "" {
		clauses = append(clauses, "(TRIM(source) = ? OR TRIM(auth_index) = ? OR TRIM(provider) = ?)")
		args = append(args, source, source, source)
	}
	switch strings.TrimSpace(filter.Result) {
	case "success":
		clauses = append(clauses, "failed = 0")
	case "failed":
		clauses = append(clauses, "failed = 1")
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (s *Store) listDistinct(ctx context.Context, column string, filter UsageEventsFilter) ([]string, error) {
	if column != "model" && column != "source" {
		return nil, fmt.Errorf("unsupported distinct column %q", column)
	}
	where, args := usageEventsWhere(UsageEventsFilter{
		StartTime: filter.StartTime,
		EndTime:   filter.EndTime,
	})
	predicate := "TRIM(" + column + ") <> ''"
	if where == "" {
		where = " WHERE " + predicate
	} else {
		where += " AND " + predicate
	}
	rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT TRIM("+column+") FROM usage_events"+where+" ORDER BY TRIM("+column+") ASC", args...)
	if err != nil {
		return nil, fmt.Errorf("usage store: list %s facets: %w", column, err)
	}
	defer rows.Close()
	return scanStrings(rows, column)
}

func scanStrings(rows *sql.Rows, label string) ([]string, error) {
	values := make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, fmt.Errorf("usage store: scan %s facet: %w", label, err)
		}
		value = strings.TrimSpace(value)
		if value != "" {
			values = append(values, value)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("usage store: read %s facets: %w", label, err)
	}
	return values, nil
}

func gzipBytes(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gunzipBytes(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	out, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeAuthType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "api_key" {
		return "apikey"
	}
	return value
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		t = time.Now().UTC()
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func formatOptionalTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.UTC()
	}
	return time.Time{}
}
