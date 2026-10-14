package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"nstock/internal/config"
	"nstock/internal/database"
	"nstock/internal/market"
	"nstock/internal/monitor"
	"nstock/internal/service"

	"github.com/gin-gonic/gin"
)

type fakeSrc struct{}

func (fakeSrc) Quotes(_ context.Context, secIDs []string) ([]market.Quote, error) {
	out := make([]market.Quote, 0, len(secIDs))
	for _, id := range secIDs {
		code := id
		if i := len(id) - 6; i >= 0 {
			code = id[len(id)-6:]
		}
		mkt := "sz"
		if len(id) > 2 && id[0] == '1' {
			mkt = "sh"
		}
		out = append(out, market.Quote{
			SecID: id, Code: code, Name: "测试证券", Market: mkt,
			Price: 10, Change: 0.2, ChangePct: 2, Open: 9.8, High: 10.2, Low: 9.7, PreClose: 9.8,
			Volume: 1000, Amount: 10000,
		})
	}
	return out, nil
}

func (fakeSrc) Kline(_ context.Context, secID string, limit int) (market.Kline, error) {
	if limit <= 0 {
		limit = 60
	}
	start := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	bars := make([]market.Bar, limit)
	for i := range bars {
		price := 10 + float64(i)*0.08
		bars[i] = market.Bar{
			Date: start.AddDate(0, 0, i).Format("2006-01-02"),
			Open: price, High: price + 0.1, Low: price - 0.1, Close: price,
			Volume: 1000 + float64(i)*10, Amount: price * 1000,
		}
	}
	code := secID
	if len(secID) >= 6 {
		code = secID[len(secID)-6:]
	}
	return market.Kline{Code: code, Name: "测试证券", Bars: bars}, nil
}

func (fakeSrc) Search(_ context.Context, query string) ([]market.Hit, error) {
	code := "600000"
	if len(query) == 6 {
		code = query
	}
	mkt := market.InferMarket(code)
	return []market.Hit{{
		Code: code, Name: "浦发银行", Market: mkt, SecID: market.SecID(code, mkt), KindName: "沪A",
	}}, nil
}

func testEngine(t *testing.T) *gin.Engine {
	t.Helper()
	cfg := config.Config{
		DBPath:          filepath.Join(t.TempDir(), "t.db"),
		WebDir:          filepath.Join(t.TempDir(), "missing"),
		MonitorInterval: time.Hour,
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := service.Seed(db); err != nil {
		t.Fatal(err)
	}
	svc := service.New(db, fakeSrc{})
	return NewEngine(svc, monitor.New(svc, cfg), cfg)
}

func call(h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	var buf *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		buf = bytes.NewReader(raw)
	} else {
		buf = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestPaperTradePredictAndMonitor(t *testing.T) {
	h := testEngine(t)
	if rec := call(h, http.MethodGet, "/api/health", nil); rec.Code != 200 {
		t.Fatalf("health %d", rec.Code)
	}

	rec := call(h, http.MethodPost, "/api/watchlist", map[string]string{"code": "600000"})
	if rec.Code != 200 {
		t.Fatalf("watch %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h, http.MethodPost, "/api/orders", map[string]any{"code": "600000", "side": "buy", "qty": 100})
	if rec.Code != 200 {
		t.Fatalf("buy %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, "/api/orders", map[string]any{"code": "600000", "side": "buy", "qty": 50})
	if rec.Code == 200 {
		t.Fatal("odd lot should fail")
	}

	rec = call(h, http.MethodGet, "/api/account", nil)
	var account struct {
		Cash      float64 `json:"cash"`
		Positions []struct {
			Qty int `json:"qty"`
		} `json:"positions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &account); err != nil || len(account.Positions) != 1 || account.Positions[0].Qty != 100 {
		t.Fatalf("account %s", rec.Body.String())
	}
	if account.Cash >= 1_000_000 {
		t.Fatalf("cash not reduced: %v", account.Cash)
	}

	rec = call(h, http.MethodPost, "/api/predict/600519", nil)
	if rec.Code != 200 {
		t.Fatalf("predict %d %s", rec.Code, rec.Body.String())
	}
	var prediction struct {
		Direction string `json:"direction"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &prediction)
	if prediction.Direction != "up" {
		t.Fatalf("direction %s", rec.Body.String())
	}

	rec = call(h, http.MethodPost, "/api/strategies", map[string]any{
		"type": "ma_cross", "code": "600519",
		"params": map[string]any{"fast": 5, "slow": 20, "positionPct": 0.2},
	})
	if rec.Code != 200 {
		t.Fatalf("strategy %d %s", rec.Code, rec.Body.String())
	}
	var strategy struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &strategy)
	rec = call(h, http.MethodPost, fmt.Sprintf("/api/strategies/%d/backtest", strategy.ID), nil)
	if rec.Code != 200 {
		t.Fatalf("backtest %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h, http.MethodPost, "/api/alerts", map[string]any{"code": "600519", "kind": "price_above", "threshold": 1})
	if rec.Code != 200 {
		t.Fatalf("alert %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, "/api/monitor/scan", nil)
	var scan struct {
		Fired int `json:"fired"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &scan)
	if rec.Code != 200 || scan.Fired != 1 {
		t.Fatalf("scan %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, "/api/monitor/scan", nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &scan)
	if scan.Fired != 0 {
		t.Fatalf("latched scan fired again: %s", rec.Body.String())
	}

	rec = call(h, http.MethodPost, "/api/orders", map[string]any{"code": "600000", "side": "sell", "qty": 100})
	if rec.Code != 200 {
		t.Fatalf("sell %d %s", rec.Code, rec.Body.String())
	}
}
