package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nstock/internal/auth"
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
	return testEngineDir(t, filepath.Join(t.TempDir(), "missing"))
}

func testEngineDir(t *testing.T, webDir string) *gin.Engine {
	t.Helper()
	cfg := config.Config{
		DBPath:          filepath.Join(t.TempDir(), "t.db"),
		WebDir:          webDir,
		MonitorInterval: time.Hour,
		Token:           token,
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
	identity := auth.New(db)
	if err := auth.Seed(db); err != nil {
		t.Fatal(err)
	}
	return NewEngine(svc, monitor.New(svc, cfg), cfg, identity)
}

func call(h http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	var buf *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		buf = bytes.NewReader(raw)
	} else {
		buf = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func envelopeData(t *testing.T, rec *httptest.ResponseRecorder) json.RawMessage {
	t.Helper()
	var body struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json %s", rec.Body.String())
	}
	if body.Code != 200 {
		t.Fatalf("code %d %s", body.Code, rec.Body.String())
	}
	return body.Data
}

func TestPaperTradePredictAndMonitor(t *testing.T) {
	h := testEngine(t)
	rec := call(h, http.MethodGet, "/api/v1/system/health", nil, "")
	if rec.Code == 200 {
		t.Fatal("business api must not use GET")
	}
	rec = call(h, http.MethodPost, "/api/v1/system/health", map[string]any{}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated health %d %s", rec.Code, rec.Body.String())
	}

	token := loginAs(t, h, auth.UserAdmin, auth.PassAdmin)
	authed := func(method, path string, body any) *httptest.ResponseRecorder {
		return call(h, method, path, body, token)
	}

	rec = authed(http.MethodPost, "/api/v1/system/health", map[string]any{})
	if rec.Code != 200 {
		t.Fatalf("health %d %s", rec.Code, rec.Body.String())
	}

	rec = authed(http.MethodPost, "/api/v1/watch/create", map[string]any{"tenantId": 1, "code": "600000"})
	if rec.Code == 200 {
		t.Fatal("client tenantId must be rejected")
	}

	rec = authed(http.MethodPost, "/api/v1/watch/create", map[string]string{"code": "600000"})
	if rec.Code != 200 {
		t.Fatalf("watch %d %s", rec.Code, rec.Body.String())
	}

	rec = authed(http.MethodPost, "/api/v1/order/create", map[string]any{"code": "600000", "side": "buy", "qty": 100})
	if rec.Code != 200 {
		t.Fatalf("buy %d %s", rec.Code, rec.Body.String())
	}
	rec = authed(http.MethodPost, "/api/v1/order/create", map[string]any{"code": "600000", "side": "buy", "qty": 50})
	if rec.Code == 200 {
		t.Fatal("odd lot should fail")
	}

	rec = authed(http.MethodPost, "/api/v1/account/get", map[string]any{})
	var account struct {
		Cash      float64 `json:"cash"`
		Positions []struct {
			Qty int `json:"qty"`
		} `json:"positions"`
	}
	if err := json.Unmarshal(envelopeData(t, rec), &account); err != nil || len(account.Positions) != 1 || account.Positions[0].Qty != 100 {
		t.Fatalf("account %s", rec.Body.String())
	}
	if account.Cash >= 1_000_000 {
		t.Fatalf("cash not reduced: %v", account.Cash)
	}

	rec = authed(http.MethodPost, "/api/v1/predict/execute", map[string]string{"code": "600519"})
	if rec.Code != 200 {
		t.Fatalf("predict %d %s", rec.Code, rec.Body.String())
	}
	var prediction struct {
		Direction string `json:"direction"`
	}
	_ = json.Unmarshal(envelopeData(t, rec), &prediction)
	if prediction.Direction != "up" {
		t.Fatalf("direction %s", rec.Body.String())
	}

	rec = authed(http.MethodPost, "/api/v1/strategy/create", map[string]any{
		"type": "ma_cross", "code": "600519",
		"params": map[string]any{"fast": 5, "slow": 20, "positionPct": 0.2},
	})
	if rec.Code != 200 {
		t.Fatalf("strategy %d %s", rec.Code, rec.Body.String())
	}
	var strategy struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(envelopeData(t, rec), &strategy)
	rec = authed(http.MethodPost, "/api/v1/strategy/backtest", map[string]any{"id": strategy.ID})
	if rec.Code != 200 {
		t.Fatalf("backtest %d %s", rec.Code, rec.Body.String())
	}

	rec = authed(http.MethodPost, "/api/v1/query/search", map[string]any{
		"model": "pageForOrder", "page": 1, "size": 20,
		"filters": map[string]any{"code": "600000"},
	})
	var page struct {
		ResponseType string `json:"responseType"`
		Total        int64  `json:"total"`
		Data         []struct {
			StateName string `json:"stateName"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || page.ResponseType != "page" || page.Total < 1 || page.Data[0].StateName == "" {
		t.Fatalf("query %s", rec.Body.String())
	}

	rec = authed(http.MethodPost, "/api/v1/query/search", map[string]any{
		"model":   "pageForOrder",
		"filters": map[string]any{"tenantId": 1},
	})
	if rec.Code == 200 {
		t.Fatal("query tenant filter must be rejected")
	}

	rec = authed(http.MethodPost, "/api/v1/alert/create", map[string]any{"code": "600519", "kind": "price_above", "threshold": 1})
	if rec.Code != 200 {
		t.Fatalf("alert %d %s", rec.Code, rec.Body.String())
	}
	rec = authed(http.MethodPost, "/api/v1/monitor/scan", map[string]any{})
	var scan struct {
		Fired int `json:"fired"`
	}
	_ = json.Unmarshal(envelopeData(t, rec), &scan)
	if rec.Code != 200 || scan.Fired != 1 {
		t.Fatalf("scan %d %s", rec.Code, rec.Body.String())
	}
	rec = authed(http.MethodPost, "/api/v1/monitor/scan", map[string]any{})
	_ = json.Unmarshal(envelopeData(t, rec), &scan)
	if scan.Fired != 0 {
		t.Fatalf("latched scan fired again: %s", rec.Body.String())
	}

	rec = authed(http.MethodPost, "/api/v1/order/create", map[string]any{"code": "600000", "side": "sell", "qty": 100})
	if rec.Code != 200 {
		t.Fatalf("sell %d %s", rec.Code, rec.Body.String())
	}
}

func TestEmbeddedFrontend(t *testing.T) {
	h := testEngine(t)
	rec := call(h, http.MethodGet, "/", nil)
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, "衡观") || !strings.Contains(body, `id="app"`) {
		t.Fatalf("index %d %s", rec.Code, body)
	}
	js := quotedAsset(t, body, ".js")
	css := quotedAsset(t, body, ".css")
	rec = call(h, http.MethodGet, js, nil)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("js %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	rec = call(h, http.MethodGet, css, nil)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("css %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	rec = call(h, http.MethodGet, "/quant", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `id="app"`) {
		t.Fatalf("spa %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodGet, "/quote/600000", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `id="app"`) {
		t.Fatalf("quote %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, "/api/v1/nope", map[string]any{})
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "接口不存在") {
		t.Fatalf("api %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodGet, "/assets/missing.js", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing asset %d", rec.Code)
	}
}

func TestDiskFrontendOverridesEmbed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("disk-page"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := testEngineDir(t, dir)
	rec := call(h, http.MethodGet, "/", nil)
	if rec.Code != 200 || rec.Body.String() != "disk-page" {
		t.Fatalf("index %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodGet, "/market", nil)
	if rec.Code != 200 || rec.Body.String() != "disk-page" {
		t.Fatalf("spa %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodGet, "/assets/app.js", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "console.log") {
		t.Fatalf("asset %d %s", rec.Code, rec.Body.String())
	}
}

func quotedAsset(t *testing.T, body, suffix string) string {
	t.Helper()
	needle := "/assets/"
	from := 0
	for {
		i := strings.Index(body[from:], needle)
		if i < 0 {
			t.Fatalf("no asset %s in %s", suffix, body)
		}
		i += from
		rest := body[i:]
		end := strings.IndexAny(rest, "\"'")
		if end > 0 && strings.HasSuffix(rest[:end], suffix) {
			return rest[:end]
		}
		from = i + len(needle)
	}
}
