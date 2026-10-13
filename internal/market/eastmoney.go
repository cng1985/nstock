package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Quote struct {
	SecID     string
	Code      string
	Name      string
	Market    string
	Price     float64
	Change    float64
	ChangePct float64
	Open      float64
	High      float64
	Low       float64
	PreClose  float64
	Volume    float64
	Amount    float64
}

type Bar struct {
	Date   string
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
	Amount float64
}

type Hit struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Market   string `json:"market"`
	SecID    string `json:"secId"`
	KindName string `json:"kindName"`
}

type Kline struct {
	Code string
	Name string
	Bars []Bar
}

type Provider interface {
	Quotes(ctx context.Context, secIDs []string) ([]Quote, error)
	Kline(ctx context.Context, secID string, limit int) (Kline, error)
	Search(ctx context.Context, query string) ([]Hit, error)
}

type EastMoney struct {
	client *http.Client
}

func NewEastMoney() *EastMoney {
	return &EastMoney{client: &http.Client{Timeout: 12 * time.Second}}
}

func (e *EastMoney) Quotes(ctx context.Context, secIDs []string) ([]Quote, error) {
	if len(secIDs) == 0 {
		return nil, fmt.Errorf("没有可查询的证券")
	}
	q := url.Values{}
	q.Set("fltt", "2")
	q.Set("invt", "2")
	q.Set("fields", "f2,f3,f4,f5,f6,f12,f13,f14,f15,f16,f17,f18")
	q.Set("secids", strings.Join(secIDs, ","))
	u := "https://push2delay.eastmoney.com/api/qt/ulist.np/get?" + q.Encode()
	body, err := e.get(ctx, u)
	if err != nil {
		return nil, err
	}
	return parseQuotes(body)
}

func (e *EastMoney) Kline(ctx context.Context, secID string, limit int) (Kline, error) {
	if limit <= 0 {
		limit = 180
	}
	if limit > 800 {
		limit = 800
	}
	q := url.Values{}
	q.Set("secid", secID)
	q.Set("klt", "101")
	q.Set("fqt", "1")
	q.Set("lmt", strconv.Itoa(limit))
	q.Set("end", "20500101")
	q.Set("fields1", "f1,f2,f3,f4,f5,f6")
	q.Set("fields2", "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61")
	u := "https://push2his.eastmoney.com/api/qt/stock/kline/get?" + q.Encode()
	body, err := e.get(ctx, u)
	if err != nil {
		return Kline{}, err
	}
	return parseKline(body)
}

func (e *EastMoney) Search(ctx context.Context, query string) ([]Hit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("请输入代码或名称")
	}
	q := url.Values{}
	q.Set("input", query)
	q.Set("type", "14")
	q.Set("count", "12")
	u := "https://searchadapter.eastmoney.com/api/suggest/get?" + q.Encode()
	body, err := e.get(ctx, u)
	if err != nil {
		return nil, err
	}
	return parseSearch(body)
}

func (e *EastMoney) get(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("行情源不可达: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("行情源返回 %d", resp.StatusCode)
	}
	return body, nil
}

type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	if string(b) == "null" || string(b) == `"-"` || string(b) == `""` {
		*f = 0
		return nil
	}
	var n float64
	if err := json.Unmarshal(b, &n); err == nil {
		*f = flexFloat(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		*f = 0
		return nil
	}
	n, _ = strconv.ParseFloat(s, 64)
	*f = flexFloat(n)
	return nil
}

func parseQuotes(body []byte) ([]Quote, error) {
	var raw struct {
		Data struct {
			Diff json.RawMessage `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("行情数据解析失败")
	}
	rows, err := decodeDiff(raw.Data.Diff)
	if err != nil {
		return nil, err
	}
	out := make([]Quote, 0, len(rows))
	for _, row := range rows {
		if row.Code == "" || float64(row.Price) == 0 {
			continue
		}
		market := "sz"
		if row.Market == 1 {
			market = "sh"
		}
		if InferMarket(row.Code) == "bj" {
			market = "bj"
		}
		secID := SecID(row.Code, market)
		out = append(out, Quote{
			SecID:     secID,
			Code:      row.Code,
			Name:      row.Name,
			Market:    market,
			Price:     float64(row.Price),
			Change:    float64(row.Change),
			ChangePct: float64(row.Pct),
			Open:      float64(row.Open),
			High:      float64(row.High),
			Low:       float64(row.Low),
			PreClose:  float64(row.Pre),
			Volume:    float64(row.Volume),
			Amount:    float64(row.Amount),
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("行情源暂无数据")
	}
	return out, nil
}

type quoteRow struct {
	Price  flexFloat `json:"f2"`
	Pct    flexFloat `json:"f3"`
	Change flexFloat `json:"f4"`
	Volume flexFloat `json:"f5"`
	Amount flexFloat `json:"f6"`
	Code   string    `json:"f12"`
	Market int       `json:"f13"`
	Name   string    `json:"f14"`
	High   flexFloat `json:"f15"`
	Low    flexFloat `json:"f16"`
	Open   flexFloat `json:"f17"`
	Pre    flexFloat `json:"f18"`
}

func decodeDiff(raw json.RawMessage) ([]quoteRow, error) {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return nil, fmt.Errorf("行情源暂无数据")
	}
	if raw[0] == '[' {
		var rows []quoteRow
		if err := json.Unmarshal(raw, &rows); err != nil {
			return nil, fmt.Errorf("行情数据解析失败")
		}
		return rows, nil
	}
	var obj map[string]quoteRow
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("行情数据解析失败")
	}
	rows := make([]quoteRow, 0, len(obj))
	for _, row := range obj {
		rows = append(rows, row)
	}
	return rows, nil
}

func parseKline(body []byte) (Kline, error) {
	var raw struct {
		Data struct {
			Code   string   `json:"code"`
			Name   string   `json:"name"`
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Kline{}, fmt.Errorf("K线数据解析失败")
	}
	if raw.Data.Code == "" || len(raw.Data.Klines) == 0 {
		return Kline{}, fmt.Errorf("行情源暂无K线")
	}
	bars := make([]Bar, 0, len(raw.Data.Klines))
	for _, line := range raw.Data.Klines {
		bar, err := parseKlineLine(line)
		if err != nil {
			continue
		}
		bars = append(bars, bar)
	}
	if len(bars) == 0 {
		return Kline{}, fmt.Errorf("行情源暂无K线")
	}
	return Kline{Code: raw.Data.Code, Name: raw.Data.Name, Bars: bars}, nil
}

func parseKlineLine(line string) (Bar, error) {
	parts := strings.Split(line, ",")
	if len(parts) < 7 {
		return Bar{}, fmt.Errorf("bad kline")
	}
	num := func(i int) float64 {
		n, _ := strconv.ParseFloat(parts[i], 64)
		return n
	}
	return Bar{
		Date:   parts[0],
		Open:   num(1),
		Close:  num(2),
		High:   num(3),
		Low:    num(4),
		Volume: num(5),
		Amount: num(6),
	}, nil
}

func parseSearch(body []byte) ([]Hit, error) {
	var raw struct {
		QuotationCodeTable struct {
			Data []struct {
				Code             string `json:"Code"`
				Name             string `json:"Name"`
				QuoteID          string `json:"QuoteID"`
				Classify         string `json:"Classify"`
				SecurityTypeName string `json:"SecurityTypeName"`
				MktNum           string `json:"MktNum"`
			} `json:"Data"`
		} `json:"QuotationCodeTable"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("搜索结果解析失败")
	}
	out := make([]Hit, 0, len(raw.QuotationCodeTable.Data))
	for _, row := range raw.QuotationCodeTable.Data {
		if row.Classify != "" && row.Classify != "AStock" && row.Classify != "Index" {
			continue
		}
		if row.Classify == "" && !strings.Contains(row.SecurityTypeName, "A") && !strings.Contains(row.SecurityTypeName, "指") {
			continue
		}
		code := row.Code
		market := InferMarket(code)
		if row.MktNum == "1" {
			market = "sh"
		}
		if InferMarket(code) == "bj" {
			market = "bj"
		}
		secID := row.QuoteID
		if secID == "" {
			secID = SecID(code, market)
		}
		out = append(out, Hit{
			Code:     code,
			Name:     row.Name,
			Market:   market,
			SecID:    secID,
			KindName: row.SecurityTypeName,
		})
	}
	return out, nil
}
