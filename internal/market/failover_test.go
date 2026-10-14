package market

import (
	"context"
	"errors"
	"testing"
	"time"
)

type scriptSrc struct {
	quotesErr error
	klineErr  error
	searchErr error
	quotesN   int
	klineN    int
	searchN   int
}

func (s *scriptSrc) Quotes(_ context.Context, secIDs []string) ([]Quote, error) {
	s.quotesN++
	if s.quotesErr != nil {
		return nil, s.quotesErr
	}
	out := make([]Quote, 0, len(secIDs))
	for _, id := range secIDs {
		out = append(out, Quote{SecID: id, Code: "600519", Name: "线上茅台", Market: "sh", Price: 1600, PreClose: 1580})
	}
	return out, nil
}

func (s *scriptSrc) Kline(_ context.Context, secID string, limit int) (Kline, error) {
	s.klineN++
	if s.klineErr != nil {
		return Kline{}, s.klineErr
	}
	return Kline{Code: secID, Name: "线上", Bars: []Bar{{Date: "2026-10-13", Close: 10, Open: 10, High: 11, Low: 9}}}, nil
}

func (s *scriptSrc) Search(_ context.Context, query string) ([]Hit, error) {
	s.searchN++
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	return []Hit{{Code: "600519", Name: "线上茅台", Market: "sh", SecID: "1.600519", KindName: "沪A"}}, nil
}

func TestFailoverSwitchesToStartupAndHolds(t *testing.T) {
	primary := &scriptSrc{quotesErr: errors.New("行情源不可达: timeout"), klineErr: errors.New("行情源不可达"), searchErr: errors.New("行情源不可达")}
	fb := ComposeFailover(primary, NewStartup())
	now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	fb.now = func() time.Time { return now }

	quotes, err := fb.Quotes(context.Background(), []string{"1.600519"})
	if err != nil || len(quotes) != 1 || quotes[0].Name != "贵州茅台" || quotes[0].Price <= 0 {
		t.Fatalf("quotes %v %+v", err, quotes)
	}
	status := fb.Status()
	if !status.Fallback || status.Name != SourceStartup || status.Label != "启动数据源" {
		t.Fatalf("status %+v", status)
	}

	before := primary.quotesN
	if _, err = fb.Quotes(context.Background(), []string{"0.000001"}); err != nil {
		t.Fatal(err)
	}
	if primary.quotesN != before {
		t.Fatalf("primary called during hold: %d -> %d", before, primary.quotesN)
	}

	hits, err := fb.Search(context.Background(), "茅台")
	if err != nil || len(hits) != 1 || hits[0].Code != "600519" {
		t.Fatalf("search %v %+v", err, hits)
	}
	k, err := fb.Kline(context.Background(), "1.601888", 30)
	if err != nil || k.Name != "中国中免" || len(k.Bars) != 30 || k.Bars[len(k.Bars)-1].Close <= 0 {
		t.Fatalf("kline %v %+v", err, k)
	}
	if primary.klineN != 0 || primary.searchN != 0 {
		t.Fatalf("held source still probed primary kline=%d search=%d", primary.klineN, primary.searchN)
	}
}

func TestFailoverReturnsToPrimaryAfterHold(t *testing.T) {
	primary := &scriptSrc{quotesErr: errors.New("行情源返回 502")}
	fb := ComposeFailover(primary, NewStartup())
	now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	fb.now = func() time.Time { return now }
	if _, err := fb.Quotes(context.Background(), []string{"1.600519"}); err != nil {
		t.Fatal(err)
	}

	now = now.Add(failoverHold + time.Second)
	primary.quotesErr = nil
	quotes, err := fb.Quotes(context.Background(), []string{"1.600519"})
	if err != nil || quotes[0].Name != "线上茅台" {
		t.Fatalf("recovered %v %+v", err, quotes)
	}
	if fb.Status().Fallback {
		t.Fatal("expected primary")
	}
}

func TestFailoverKeepsValidationErrors(t *testing.T) {
	primary := &scriptSrc{searchErr: errors.New("请输入代码或名称")}
	fb := ComposeFailover(primary, NewStartup())
	if _, err := fb.Search(context.Background(), "茅台"); err == nil || fb.Status().Fallback {
		t.Fatalf("err %v status %+v", err, fb.Status())
	}
}

func TestStartupSearchQuoteAndBars(t *testing.T) {
	src := NewStartup()
	hits, err := src.Search(context.Background(), "中免")
	if err != nil || len(hits) != 1 || hits[0].SecID != "1.601888" {
		t.Fatalf("search %v %+v", err, hits)
	}
	unknown, err := src.Search(context.Background(), "601111")
	if err != nil || unknown[0].Code != "601111" || unknown[0].Name != "601111" {
		t.Fatalf("unknown %v %+v", err, unknown)
	}
	quotes, err := src.Quotes(context.Background(), []string{"1.000001", "0.000001"})
	if err != nil || len(quotes) != 2 || quotes[0].Name != "上证指数" || quotes[1].Name != "平安银行" {
		t.Fatalf("quotes %v %+v", err, quotes)
	}
	k, err := src.Kline(context.Background(), "0.300750", 40)
	if err != nil || len(k.Bars) != 40 {
		t.Fatalf("bars %v %d", err, len(k.Bars))
	}
	for _, bar := range k.Bars {
		if bar.Low <= 0 || bar.Low > bar.Open || bar.Low > bar.Close || bar.High < bar.Open || bar.High < bar.Close {
			t.Fatalf("ohlc %+v", bar)
		}
	}
	if k.Bars[len(k.Bars)-1].Close != 250 {
		t.Fatalf("last close %v", k.Bars[len(k.Bars)-1].Close)
	}
}
