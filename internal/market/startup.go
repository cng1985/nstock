package market

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"strings"
	"time"
)

const (
	SourceEastMoney = "eastmoney"
	SourceStartup   = "startup"
	FallbackNotice  = "东方财富行情不可用，已自动切换到启动数据源。当前为内置样本行情，源恢复后会自动切回。"
)

// SourceStatus 是当前实际使用的行情源。
type SourceStatus struct {
	Name     string
	Label    string
	Fallback bool
}

// Reporter 报告最近一次请求落到了哪个行情源。
type Reporter interface {
	Status() SourceStatus
}

func SourceLabel(name string) string {
	if name == SourceStartup {
		return "启动数据源"
	}
	return "东方财富公开行情"
}

type sample struct {
	SecID  string
	Code   string
	Name   string
	Market string
	Price  float64
	Index  bool
}

// Startup 是随进程启动的内置行情。东方财富不可达时用它继续搜索、报价和日 K。
type Startup struct {
	bySec  map[string]sample
	stocks []sample
}

func NewStartup() *Startup {
	rows := startupSamples()
	bySec := make(map[string]sample, len(rows))
	stocks := make([]sample, 0, len(rows))
	for _, row := range rows {
		bySec[row.SecID] = row
		if !row.Index {
			stocks = append(stocks, row)
		}
	}
	return &Startup{bySec: bySec, stocks: stocks}
}

func (s *Startup) Quotes(_ context.Context, secIDs []string) ([]Quote, error) {
	if len(secIDs) == 0 {
		return nil, fmt.Errorf("没有可查询的证券")
	}
	out := make([]Quote, 0, len(secIDs))
	seen := map[string]struct{}{}
	for _, id := range secIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		item, ok := s.lookup(id)
		if !ok {
			continue
		}
		bars := sampleBars(item, 2)
		last := bars[len(bars)-1]
		prev := last.Close
		if len(bars) > 1 {
			prev = bars[len(bars)-2].Close
		}
		change := round2(last.Close - prev)
		pct := 0.0
		if prev > 0 {
			pct = round2(change / prev * 100)
		}
		out = append(out, Quote{
			SecID: item.SecID, Code: item.Code, Name: item.Name, Market: item.Market,
			Price: last.Close, Change: change, ChangePct: pct,
			Open: last.Open, High: last.High, Low: last.Low, PreClose: prev,
			Volume: last.Volume, Amount: last.Amount,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("行情源暂无数据")
	}
	return out, nil
}

func (s *Startup) Kline(_ context.Context, secID string, limit int) (Kline, error) {
	if limit <= 0 {
		limit = 180
	}
	if limit > 800 {
		limit = 800
	}
	item, ok := s.lookup(secID)
	if !ok {
		return Kline{}, fmt.Errorf("行情源暂无K线")
	}
	return Kline{Code: item.Code, Name: item.Name, Bars: sampleBars(item, limit)}, nil
}

func (s *Startup) Search(_ context.Context, query string) ([]Hit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("请输入代码或名称")
	}
	out := make([]Hit, 0, 8)
	if code, err := NormalizeCode(query); err == nil {
		if item, ok := s.stockByCode(code); ok {
			out = append(out, hitOf(item))
		} else {
			market := InferMarket(code)
			out = append(out, Hit{
				Code: code, Name: code, Market: market, SecID: SecID(code, market), KindName: kindName(market),
			})
		}
		return out, nil
	}
	needle := strings.ToLower(query)
	for _, item := range s.stocks {
		if strings.Contains(strings.ToLower(item.Name), needle) || strings.Contains(item.Code, query) {
			out = append(out, hitOf(item))
			if len(out) >= 12 {
				break
			}
		}
	}
	return out, nil
}

func (s *Startup) lookup(secID string) (sample, bool) {
	if item, ok := s.bySec[secID]; ok {
		return item, true
	}
	code, market, ok := splitSecID(secID)
	if !ok {
		return sample{}, false
	}
	if item, found := s.stockByCode(code); found && item.SecID == secID {
		return item, true
	}
	return sample{
		SecID: secID, Code: code, Name: code, Market: market, Price: syntheticPrice(secID),
	}, true
}

func (s *Startup) stockByCode(code string) (sample, bool) {
	for _, item := range s.stocks {
		if item.Code == code {
			return item, true
		}
	}
	return sample{}, false
}

func hitOf(item sample) Hit {
	return Hit{
		Code: item.Code, Name: item.Name, Market: item.Market, SecID: item.SecID, KindName: kindName(item.Market),
	}
}

func kindName(market string) string {
	switch market {
	case "sh":
		return "沪A"
	case "bj":
		return "京A"
	default:
		return "深A"
	}
}

func splitSecID(secID string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(secID), ".", 2)
	if len(parts) != 2 || len(parts[1]) != 6 {
		return "", "", false
	}
	market := "sz"
	if parts[0] == "1" {
		market = "sh"
	}
	if InferMarket(parts[1]) == "bj" {
		market = "bj"
	}
	return parts[1], market, true
}

func syntheticPrice(secID string) float64 {
	return round2(6 + float64(hash64(secID)%6400)/100)
}

func sampleBars(item sample, limit int) []Bar {
	if limit < 2 {
		limit = 2
	}
	days := tradingDays(limit, time.Now())
	src := rand.New(rand.NewSource(int64(hash64(item.SecID))))
	closes := make([]float64, limit)
	base := item.Price
	if base <= 0 {
		base = syntheticPrice(item.SecID)
	}
	price := base
	for i := 0; i < limit; i++ {
		shock := (src.Float64() - 0.48) * 0.028
		price *= 1 + shock
		if price < base*0.55 {
			price = base * 0.55
		}
		if price > base*1.7 {
			price = base * 1.7
		}
		closes[i] = price
	}
	scale := base / closes[limit-1]
	for i := range closes {
		closes[i] = round2(closes[i] * scale)
	}
	closes[limit-1] = round2(base)
	bars := make([]Bar, limit)
	prev := closes[0]
	for i := range bars {
		closePx := closes[i]
		open := prev
		if i == 0 {
			open = round2(closePx * (1 + (src.Float64()-0.5)*0.01))
		}
		high := math.Max(open, closePx)
		low := math.Min(open, closePx)
		high = round2(high * (1 + 0.002 + src.Float64()*0.008))
		low = round2(low * (1 - 0.002 - src.Float64()*0.008))
		if low <= 0 {
			low = round2(math.Min(open, closePx))
		}
		if high < math.Max(open, closePx) {
			high = round2(math.Max(open, closePx))
		}
		if low > math.Min(open, closePx) {
			low = round2(math.Min(open, closePx))
		}
		volume := 60000 + float64((hash64(item.Code+days[i]) % 900000))
		if item.Index {
			volume *= 30
		}
		bars[i] = Bar{
			Date: days[i], Open: open, High: high, Low: low, Close: closePx,
			Volume: volume, Amount: round2(volume * 100 * closePx),
		}
		prev = closePx
	}
	return bars
}

func tradingDays(n int, end time.Time) []string {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	day := end.In(loc)
	out := make([]string, 0, n)
	for len(out) < n {
		if day.Weekday() != time.Saturday && day.Weekday() != time.Sunday {
			out = append(out, day.Format("2006-01-02"))
		}
		day = day.AddDate(0, 0, -1)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func hash64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func startupSamples() []sample {
	return []sample{
		{SecID: "1.000001", Code: "000001", Name: "上证指数", Market: "sh", Price: 3250, Index: true},
		{SecID: "0.399001", Code: "399001", Name: "深证成指", Market: "sz", Price: 10200, Index: true},
		{SecID: "0.399006", Code: "399006", Name: "创业板指", Market: "sz", Price: 2150, Index: true},
		{SecID: "1.000300", Code: "000300", Name: "沪深300", Market: "sh", Price: 3900, Index: true},

		{SecID: "1.600519", Code: "600519", Name: "贵州茅台", Market: "sh", Price: 1450},
		{SecID: "0.000001", Code: "000001", Name: "平安银行", Market: "sz", Price: 11.5},
		{SecID: "0.300750", Code: "300750", Name: "宁德时代", Market: "sz", Price: 250},
		{SecID: "1.601318", Code: "601318", Name: "中国平安", Market: "sh", Price: 48},
		{SecID: "0.000858", Code: "000858", Name: "五粮液", Market: "sz", Price: 130},
		{SecID: "0.002594", Code: "002594", Name: "比亚迪", Market: "sz", Price: 280},
		{SecID: "1.600036", Code: "600036", Name: "招商银行", Market: "sh", Price: 38},
		{SecID: "1.601398", Code: "601398", Name: "工商银行", Market: "sh", Price: 6.2},
		{SecID: "1.600000", Code: "600000", Name: "浦发银行", Market: "sh", Price: 9.8},
		{SecID: "0.000333", Code: "000333", Name: "美的集团", Market: "sz", Price: 70},
		{SecID: "1.600276", Code: "600276", Name: "恒瑞医药", Market: "sh", Price: 48},
		{SecID: "1.601888", Code: "601888", Name: "中国中免", Market: "sh", Price: 68},
		{SecID: "0.002415", Code: "002415", Name: "海康威视", Market: "sz", Price: 32},
		{SecID: "1.600900", Code: "600900", Name: "长江电力", Market: "sh", Price: 28},
		{SecID: "1.601012", Code: "601012", Name: "隆基绿能", Market: "sh", Price: 16},
		{SecID: "0.000651", Code: "000651", Name: "格力电器", Market: "sz", Price: 42},
		{SecID: "1.600887", Code: "600887", Name: "伊利股份", Market: "sh", Price: 28},
		{SecID: "1.601166", Code: "601166", Name: "兴业银行", Market: "sh", Price: 18},
		{SecID: "1.600030", Code: "600030", Name: "中信证券", Market: "sh", Price: 26},
		{SecID: "0.000002", Code: "000002", Name: "万科A", Market: "sz", Price: 7.2},
		{SecID: "1.601857", Code: "601857", Name: "中国石油", Market: "sh", Price: 8.6},
		{SecID: "1.600028", Code: "600028", Name: "中国石化", Market: "sh", Price: 6.4},
		{SecID: "1.601288", Code: "601288", Name: "农业银行", Market: "sh", Price: 5.1},
		{SecID: "0.300059", Code: "300059", Name: "东方财富", Market: "sz", Price: 15},
		{SecID: "0.002475", Code: "002475", Name: "立讯精密", Market: "sz", Price: 40},
		{SecID: "1.600309", Code: "600309", Name: "万华化学", Market: "sh", Price: 75},
		{SecID: "0.000725", Code: "000725", Name: "京东方A", Market: "sz", Price: 4.2},
		{SecID: "1.601919", Code: "601919", Name: "中远海控", Market: "sh", Price: 14},
		{SecID: "1.688981", Code: "688981", Name: "中芯国际", Market: "sh", Price: 52},
		{SecID: "1.603259", Code: "603259", Name: "药明康德", Market: "sh", Price: 55},
		{SecID: "0.002352", Code: "002352", Name: "顺丰控股", Market: "sz", Price: 40},
		{SecID: "1.600104", Code: "600104", Name: "上汽集团", Market: "sh", Price: 15},
		{SecID: "1.601668", Code: "601668", Name: "中国建筑", Market: "sh", Price: 6},
		{SecID: "1.600031", Code: "600031", Name: "三一重工", Market: "sh", Price: 16},
		{SecID: "0.002230", Code: "002230", Name: "科大讯飞", Market: "sz", Price: 48},
		{SecID: "0.300015", Code: "300015", Name: "爱尔眼科", Market: "sz", Price: 13},
		{SecID: "1.600585", Code: "600585", Name: "海螺水泥", Market: "sh", Price: 24},
		{SecID: "0.000568", Code: "000568", Name: "泸州老窖", Market: "sz", Price: 140},
		{SecID: "1.600809", Code: "600809", Name: "山西汾酒", Market: "sh", Price: 180},
		{SecID: "0.002304", Code: "002304", Name: "洋河股份", Market: "sz", Price: 85},
		{SecID: "1.688111", Code: "688111", Name: "金山办公", Market: "sh", Price: 280},
		{SecID: "0.300124", Code: "300124", Name: "汇川技术", Market: "sz", Price: 65},
		{SecID: "0.002714", Code: "002714", Name: "牧原股份", Market: "sz", Price: 42},
		{SecID: "1.601899", Code: "601899", Name: "紫金矿业", Market: "sh", Price: 17},
		{SecID: "1.600438", Code: "600438", Name: "通威股份", Market: "sh", Price: 20},
		{SecID: "0.002027", Code: "002027", Name: "分众传媒", Market: "sz", Price: 7},
		{SecID: "0.830799", Code: "830799", Name: "艾融软件", Market: "bj", Price: 12},
	}
}
