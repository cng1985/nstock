package market

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const failoverHold = 30 * time.Second

// Failover 先请求东方财富。源不可用时自动改用启动数据源，并在冷却结束后试探切回。
type Failover struct {
	primary Provider
	startup Provider
	now     func() time.Time

	mu       sync.Mutex
	fallback bool
	until    time.Time
}

func NewFailover(primary *EastMoney, startup *Startup) *Failover {
	return ComposeFailover(primary, startup)
}

// ComposeFailover 用任意两个行情源组装切换器，便于测试。
func ComposeFailover(primary, startup Provider) *Failover {
	return &Failover{primary: primary, startup: startup}
}

func (f *Failover) Status() SourceStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	name := SourceEastMoney
	if f.fallback {
		name = SourceStartup
	}
	return SourceStatus{Name: name, Label: SourceLabel(name), Fallback: f.fallback}
}

func (f *Failover) Quotes(ctx context.Context, secIDs []string) ([]Quote, error) {
	if f.holding() {
		return f.startup.Quotes(ctx, secIDs)
	}
	quotes, err := f.primary.Quotes(ctx, secIDs)
	if err == nil {
		f.markPrimary()
		return quotes, nil
	}
	if !shouldFallback(err) {
		return nil, err
	}
	f.markStartup(err)
	return f.startup.Quotes(ctx, secIDs)
}

func (f *Failover) Kline(ctx context.Context, secID string, limit int) (Kline, error) {
	if f.holding() {
		return f.startup.Kline(ctx, secID, limit)
	}
	kline, err := f.primary.Kline(ctx, secID, limit)
	if err == nil {
		f.markPrimary()
		return kline, nil
	}
	if !shouldFallback(err) {
		return Kline{}, err
	}
	f.markStartup(err)
	return f.startup.Kline(ctx, secID, limit)
}

func (f *Failover) Search(ctx context.Context, query string) ([]Hit, error) {
	if f.holding() {
		return f.startup.Search(ctx, query)
	}
	hits, err := f.primary.Search(ctx, query)
	if err == nil {
		f.markPrimary()
		return hits, nil
	}
	if !shouldFallback(err) {
		return nil, err
	}
	f.markStartup(err)
	return f.startup.Search(ctx, query)
}

func (f *Failover) holding() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fallback && f.clock().Before(f.until)
}

func (f *Failover) markPrimary() {
	f.mu.Lock()
	was := f.fallback
	f.fallback = false
	f.until = time.Time{}
	f.mu.Unlock()
	if was {
		slog.Info("行情源已恢复", "source", SourceEastMoney)
	}
}

func (f *Failover) markStartup(err error) {
	f.mu.Lock()
	first := !f.fallback
	f.fallback = true
	f.until = f.clock().Add(failoverHold)
	f.mu.Unlock()
	if first {
		slog.Warn("行情源不可用，切换到启动数据源", "err", err)
	}
}

func (f *Failover) clock() time.Time {
	if f.now != nil {
		return f.now()
	}
	return time.Now()
}

func shouldFallback(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "没有可查询") || strings.Contains(msg, "请输入代码或名称") {
		return false
	}
	return true
}
