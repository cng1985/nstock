package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"nstock/internal/config"
	"nstock/internal/database"
	"nstock/internal/market"
)

type downProvider struct{}

func (downProvider) Quotes(context.Context, []string) ([]market.Quote, error) {
	return nil, errors.New("行情源不可达: dial tcp")
}

func (downProvider) Kline(context.Context, string, int) (market.Kline, error) {
	return market.Kline{}, errors.New("行情源不可达: dial tcp")
}

func (downProvider) Search(context.Context, string) ([]market.Hit, error) {
	return nil, errors.New("行情源不可达: dial tcp")
}

func TestAddWatchFallsBackToStartup(t *testing.T) {
	cfg := config.Config{DBPath: filepath.Join(t.TempDir(), "t.db")}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err = Seed(db); err != nil {
		t.Fatal(err)
	}
	svc := New(db, market.ComposeFailover(downProvider{}, market.NewStartup()))

	item, err := svc.AddWatch(context.Background(), "中免")
	if err != nil {
		t.Fatal(err)
	}
	if item.Code != "601888" || item.Name != "中国中免" {
		t.Fatalf("watch %+v", item)
	}
	list, err := svc.WatchQuotes(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !list.Fallback || list.Source != market.SourceStartup || list.Warning == "" {
		t.Fatalf("list source=%s fallback=%v warning=%s", list.Source, list.Fallback, list.Warning)
	}
	var found bool
	for _, row := range list.Items {
		if row.Code == "601888" && row.Quote != nil && row.Quote.Price > 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing startup quote: %+v", list.Items)
	}
	view, err := svc.Kline(context.Background(), "601888", 40, true)
	if err != nil || len(view.Bars) < 40 || view.Name != "中国中免" {
		t.Fatalf("kline %v bars=%d name=%s", err, len(view.Bars), view.Name)
	}
	status := svc.SourceStatus()
	if !status.Fallback || status.Label != "启动数据源" {
		t.Fatalf("status %+v", status)
	}
}
