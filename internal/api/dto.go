package api

import (
	"nstock/internal/market"
	"nstock/internal/model"
	"nstock/internal/monitor"
	"nstock/internal/ncode"
	"nstock/internal/predict"
	"nstock/internal/quant"
	"nstock/internal/service"
)

type quoteDTO struct {
	SecID     string  `json:"secId"`
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Market    string  `json:"market"`
	Kind      string  `json:"kind,omitempty"`
	Price     float64 `json:"price"`
	Change    float64 `json:"change"`
	ChangePct float64 `json:"changePct"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	PreClose  float64 `json:"preClose"`
	Volume    float64 `json:"volume"`
	Amount    float64 `json:"amount"`
	UpdatedAt string  `json:"updatedAt,omitempty"`
}

type watchDTO struct {
	ID        uint   `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Market    string `json:"market"`
	SecID     string `json:"secId"`
	CreatedAt string `json:"createdAt"`
}

type watchQuoteDTO struct {
	watchDTO
	Quote *quoteDTO `json:"quote"`
}

type quoteListDTO struct {
	Items    []watchQuoteDTO `json:"items"`
	Warning  string          `json:"warning,omitempty"`
	Source   string          `json:"source,omitempty"`
	Fallback bool            `json:"fallback,omitempty"`
}

type barDTO struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
	Amount float64 `json:"amount"`
}

type klineDTO struct {
	Code   string   `json:"code"`
	Name   string   `json:"name"`
	Market string   `json:"market"`
	Bars   []barDTO `json:"bars"`
}

type reasonDTO struct {
	Name   string  `json:"name"`
	Score  float64 `json:"score"`
	Detail string  `json:"detail"`
}

type predictionDTO struct {
	ID         uint        `json:"id"`
	Code       string      `json:"code"`
	Name       string      `json:"name"`
	Direction  string      `json:"direction"`
	Confidence float64     `json:"confidence"`
	Score      float64     `json:"score"`
	Horizon    string      `json:"horizon"`
	Close      float64     `json:"close"`
	CreatedAt  string      `json:"createdAt"`
	Reasons    []reasonDTO `json:"reasons"`
}

type positionDTO struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Qty         int     `json:"qty"`
	AvgCost     float64 `json:"avgCost"`
	Last        float64 `json:"last"`
	MarketValue float64 `json:"marketValue"`
	PnL         float64 `json:"pnl"`
	PnLPct      float64 `json:"pnlPct"`
	UpdatedAt   string  `json:"updatedAt,omitempty"`
}

type accountDTO struct {
	Name        string        `json:"name"`
	Cash        float64       `json:"cash"`
	Initial     float64       `json:"initial"`
	MarketValue float64       `json:"marketValue"`
	Equity      float64       `json:"equity"`
	PnL         float64       `json:"pnl"`
	PnLPct      float64       `json:"pnlPct"`
	Positions   []positionDTO `json:"positions"`
}

type orderDTO struct {
	ID         uint    `json:"id"`
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Side       string  `json:"side"`
	Qty        int     `json:"qty"`
	Price      float64 `json:"price"`
	Amount     float64 `json:"amount"`
	Fee        float64 `json:"fee"`
	PnL        float64 `json:"pnl"`
	Status     string  `json:"status"`
	State      int     `json:"state"`
	StateName  string  `json:"stateName"`
	Source     string  `json:"source"`
	StrategyID uint    `json:"strategyId"`
	Message    string  `json:"message"`
	CreatedAt  string  `json:"createdAt"`
}

type strategyDTO struct {
	ID          uint         `json:"id"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Code        string       `json:"code"`
	StockName   string       `json:"stockName"`
	Params      quant.Params `json:"params"`
	Enabled     bool         `json:"enabled"`
	LastSignal  string       `json:"lastSignal"`
	LastMessage string       `json:"lastMessage"`
	LastRunAt   string       `json:"lastRunAt,omitempty"`
	CreatedAt   string       `json:"createdAt"`
	UpdatedAt   string       `json:"updatedAt"`
}

type backtestDTO struct {
	ID          uint          `json:"id"`
	StrategyID  uint          `json:"strategyId"`
	Code        string        `json:"code"`
	Type        string        `json:"type"`
	Initial     float64       `json:"initial"`
	FinalEquity float64       `json:"finalEquity"`
	ReturnPct   float64       `json:"returnPct"`
	MaxDrawdown float64       `json:"maxDrawdown"`
	Trades      int           `json:"trades"`
	WinRate     float64       `json:"winRate"`
	Curve       []quant.Point `json:"curve"`
	Fills       []quant.Fill  `json:"fills"`
	Note        string        `json:"note"`
	CreatedAt   string        `json:"createdAt"`
}

type alertDTO struct {
	ID        uint    `json:"id"`
	Code      string  `json:"code"`
	SecID     string  `json:"secId"`
	Name      string  `json:"name"`
	StockName string  `json:"stockName"`
	Kind      string  `json:"kind"`
	Threshold float64 `json:"threshold"`
	Enabled   bool    `json:"enabled"`
	Latched   bool    `json:"latched"`
	State     int     `json:"state"`
	StateName string  `json:"stateName"`
	LastPrice float64 `json:"lastPrice"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
}

type eventDTO struct {
	ID        uint    `json:"id"`
	RuleID    uint    `json:"ruleId"`
	Code      string  `json:"code"`
	StockName string  `json:"stockName"`
	Message   string  `json:"message"`
	Price     float64 `json:"price"`
	CreatedAt string  `json:"createdAt"`
}

type runDTO struct {
	StrategyID uint      `json:"strategyId"`
	Name       string    `json:"name"`
	Code       string    `json:"code"`
	Acted      bool      `json:"acted"`
	Signal     string    `json:"signal"`
	Message    string    `json:"message"`
	Order      *orderDTO `json:"order,omitempty"`
}

type scanDTO struct {
	Checked   int    `json:"checked"`
	Fired     int    `json:"fired"`
	ScannedAt string `json:"scannedAt"`
}

type monitorDTO struct {
	Running         bool   `json:"running"`
	IntervalSeconds int    `json:"intervalSeconds"`
	LastScan        string `json:"lastScan,omitempty"`
	LastError       string `json:"lastError"`
	Checked         int    `json:"checked"`
	Fired           int    `json:"fired"`
}

func toQuote(row model.QuoteSnap) quoteDTO {
	return quoteDTO{
		SecID: row.SecID, Code: row.Code, Name: row.Name, Market: row.Market, Kind: row.Kind,
		Price: row.Price, Change: row.Change, ChangePct: row.ChangePct,
		Open: row.Open, High: row.High, Low: row.Low, PreClose: row.PreClose,
		Volume: row.Volume, Amount: row.Amount, UpdatedAt: ncode.FormatTime(row.UpdatedAt),
	}
}

func toQuotes(rows []model.QuoteSnap) []quoteDTO {
	out := make([]quoteDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toQuote(row))
	}
	return out
}

func toWatch(row model.WatchItem) watchDTO {
	return watchDTO{
		ID: row.ID, Code: row.Code, Name: row.Name, Market: row.Market, SecID: row.SecID,
		CreatedAt: ncode.FormatTime(row.CreatedAt),
	}
}

func toWatches(rows []model.WatchItem) []watchDTO {
	out := make([]watchDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toWatch(row))
	}
	return out
}

func toWatchQuotes(list service.WatchList) quoteListDTO {
	items := make([]watchQuoteDTO, 0, len(list.Items))
	for _, item := range list.Items {
		row := watchQuoteDTO{watchDTO: toWatch(item.WatchItem)}
		if item.Quote != nil {
			quote := toQuote(*item.Quote)
			row.Quote = &quote
		}
		items = append(items, row)
	}
	return quoteListDTO{Items: items, Warning: list.Warning, Source: list.Source, Fallback: list.Fallback}
}

func toKline(view service.KlineView) klineDTO {
	bars := make([]barDTO, 0, len(view.Bars))
	for _, bar := range view.Bars {
		bars = append(bars, barDTO{
			Date: bar.Date, Open: bar.Open, High: bar.High, Low: bar.Low, Close: bar.Close,
			Volume: bar.Volume, Amount: bar.Amount,
		})
	}
	return klineDTO{Code: view.Code, Name: view.Name, Market: view.Market, Bars: bars}
}

func toReasons(rows []predict.Reason) []reasonDTO {
	out := make([]reasonDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, reasonDTO{Name: row.Name, Score: row.Score, Detail: row.Detail})
	}
	return out
}

func toPrediction(row service.PredictionView) predictionDTO {
	return predictionDTO{
		ID: row.ID, Code: row.Code, Name: row.Name, Direction: row.Direction,
		Confidence: row.Confidence, Score: row.Score, Horizon: row.Horizon, Close: row.Close,
		CreatedAt: ncode.FormatTime(row.CreatedAt), Reasons: toReasons(row.ReasonItems),
	}
}

func toPredictions(rows []service.PredictionView) []predictionDTO {
	out := make([]predictionDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPrediction(row))
	}
	return out
}

func toAccount(view service.AccountView) accountDTO {
	positions := make([]positionDTO, 0, len(view.Positions))
	for _, pos := range view.Positions {
		positions = append(positions, positionDTO{
			Code: pos.Code, Name: pos.Name, Qty: pos.Qty, AvgCost: pos.AvgCost,
			Last: pos.Last, MarketValue: pos.MarketValue, PnL: pos.PnL, PnLPct: pos.PnLPct,
			UpdatedAt: ncode.FormatTime(pos.UpdatedAt),
		})
	}
	return accountDTO{
		Name: view.Name, Cash: view.Cash, Initial: view.Initial, MarketValue: view.MarketValue,
		Equity: view.Equity, PnL: view.PnL, PnLPct: view.PnLPct, Positions: positions,
	}
}

func toOrder(row model.Order) orderDTO {
	state, name := 0, "拒绝"
	if row.Status == "filled" {
		state, name = 1, "成交"
	}
	return orderDTO{
		ID: row.ID, Code: row.Code, Name: row.Name, Side: row.Side, Qty: row.Qty,
		Price: row.Price, Amount: row.Amount, Fee: row.Fee, PnL: row.PnL,
		Status: row.Status, State: state, StateName: name, Source: row.Source,
		StrategyID: row.StrategyID, Message: row.Message, CreatedAt: ncode.FormatTime(row.CreatedAt),
	}
}

func toOrders(rows []model.Order) []orderDTO {
	out := make([]orderDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toOrder(row))
	}
	return out
}

func toStrategy(row service.StrategyView) strategyDTO {
	dto := strategyDTO{
		ID: row.ID, Name: row.Name, Type: row.Type, Code: row.Code, StockName: row.StockName,
		Params: row.ParamSet, Enabled: row.Enabled, LastSignal: row.LastSignal, LastMessage: row.LastMessage,
		CreatedAt: ncode.FormatTime(row.CreatedAt), UpdatedAt: ncode.FormatTime(row.UpdatedAt),
	}
	if row.LastRunAt != nil {
		dto.LastRunAt = ncode.FormatTime(*row.LastRunAt)
	}
	return dto
}

func toStrategies(rows []service.StrategyView) []strategyDTO {
	out := make([]strategyDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toStrategy(row))
	}
	return out
}

func toBacktest(row service.BacktestView) backtestDTO {
	curve := row.Curve
	if curve == nil {
		curve = []quant.Point{}
	}
	fills := row.Fills
	if fills == nil {
		fills = []quant.Fill{}
	}
	return backtestDTO{
		ID: row.ID, StrategyID: row.StrategyID, Code: row.Code, Type: row.Type,
		Initial: row.Initial, FinalEquity: row.FinalEquity, ReturnPct: row.ReturnPct,
		MaxDrawdown: row.MaxDrawdown, Trades: row.Trades, WinRate: row.WinRate,
		Curve: curve, Fills: fills, Note: row.Note, CreatedAt: ncode.FormatTime(row.CreatedAt),
	}
}

func toBacktests(rows []service.BacktestView) []backtestDTO {
	out := make([]backtestDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toBacktest(row))
	}
	return out
}

func toAlert(row model.AlertRule) alertDTO {
	state, name := 1, "监视中"
	switch {
	case !row.Enabled:
		state, name = 0, "停用"
	case row.Latched:
		state, name = 2, "已触发"
	}
	return alertDTO{
		ID: row.ID, Code: row.Code, SecID: row.SecID, Name: row.Name, StockName: row.StockName,
		Kind: row.Kind, Threshold: row.Threshold, Enabled: row.Enabled, Latched: row.Latched,
		State: state, StateName: name, LastPrice: row.LastPrice,
		CreatedAt: ncode.FormatTime(row.CreatedAt), UpdatedAt: ncode.FormatTime(row.UpdatedAt),
	}
}

func toAlerts(rows []model.AlertRule) []alertDTO {
	out := make([]alertDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAlert(row))
	}
	return out
}

func toEvents(rows []model.AlertEvent) []eventDTO {
	out := make([]eventDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, eventDTO{
			ID: row.ID, RuleID: row.RuleID, Code: row.Code, StockName: row.StockName,
			Message: row.Message, Price: row.Price, CreatedAt: ncode.FormatTime(row.CreatedAt),
		})
	}
	return out
}

func toRun(row service.RunResult) runDTO {
	dto := runDTO{
		StrategyID: row.StrategyID, Name: row.Name, Code: row.Code,
		Acted: row.Acted, Signal: row.Signal, Message: row.Message,
	}
	if row.Order != nil {
		order := toOrder(*row.Order)
		dto.Order = &order
	}
	return dto
}

func toRuns(rows []service.RunResult) []runDTO {
	out := make([]runDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRun(row))
	}
	return out
}

func toScan(row service.ScanResult) scanDTO {
	return scanDTO{Checked: row.Checked, Fired: row.Fired, ScannedAt: ncode.FormatTime(row.ScannedAt)}
}

func toMonitor(row monitor.Status) monitorDTO {
	dto := monitorDTO{
		Running: row.Running, IntervalSeconds: row.IntervalSeconds,
		LastError: row.LastError, Checked: row.Checked, Fired: row.Fired,
	}
	if row.LastScan != nil {
		dto.LastScan = ncode.FormatTime(*row.LastScan)
	}
	return dto
}

func toHits(rows []market.Hit) []market.Hit {
	if rows == nil {
		return []market.Hit{}
	}
	return rows
}
