package usage

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type blockingUsagePlugin struct {
	started chan struct{}
	release chan struct{}
	count   atomic.Int64
}

func (p *blockingUsagePlugin) HandleUsage(context.Context, Record) {
	p.count.Add(1)
	select {
	case p.started <- struct{}{}:
	default:
	}
	<-p.release
}

func TestManagerStopWaitsForQueuedUsage(t *testing.T) {
	manager := NewManager(2)
	plugin := &blockingUsagePlugin{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	manager.Register(plugin)
	manager.Publish(context.Background(), Record{Model: "first"})
	manager.Publish(context.Background(), Record{Model: "second"})

	select {
	case <-plugin.started:
	case <-time.After(time.Second):
		t.Fatal("usage dispatch did not start")
	}

	stopped := make(chan struct{})
	go func() {
		manager.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
		t.Fatal("Stop() returned before the active usage write completed")
	case <-time.After(20 * time.Millisecond):
	}

	close(plugin.release)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop() did not finish after queued usage drained")
	}
	if got := plugin.count.Load(); got != 2 {
		t.Fatalf("delivered records = %d, want 2", got)
	}
}
