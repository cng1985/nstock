package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"nstock/internal/market"
	"nstock/internal/model"
	"nstock/internal/quant"
)

type PositionView struct {
	model.Position
	Last        float64 `json:"last"`
	MarketValue float64 `json:"marketValue"`
	PnL         float64 `json:"pnl"`
	PnLPct      float64 `json:"pnlPct"`
}

type AccountView struct {
	Name        string         `json:"name"`
	Cash        float64        `json:"cash"`
	Initial     float64        `json:"initial"`
	MarketValue float64        `json:"marketValue"`
	Equity      float64        `json:"equity"`
	PnL         float64        `json:"pnl"`
	PnLPct      float64        `json:"pnlPct"`
	Positions   []PositionView `json:"positions"`
}

type OrderInput struct {
	Code       string
	Side       string
	Qty        int
	Source     string
	StrategyID uint
}

type StrategyInput struct {
	Name    string
	Type    string
	Code    string
	Params  quant.Params
	Enabled bool
}

type StrategyPatch struct {
	Name    *string
	Code    *string
	Enabled *bool
	Params  *quant.Params
}

type StrategyView struct {
	model.Strategy
	ParamSet quant.Params `json:"params"`
}

type BacktestView struct {
	model.Backtest
	Curve []quant.Point `json:"curve"`
	Fills []quant.Fill  `json:"fills"`
	Note  string        `json:"note"`
}

type RunResult struct {
	StrategyID uint         `json:"strategyId"`
	Name       string       `json:"name"`
	Code       string       `json:"code"`
	Acted      bool         `json:"acted"`
	Signal     string       `json:"signal"`
	Message    string       `json:"message"`
	Order      *model.Order `json:"order,omitempty"`
}

var tradeMu sync.Mutex

const backtestNote = "按日线收盘价模拟成交，计入佣金和卖出印花税，只做多、不做空。这不是实盘报单。"

func (s *Service) Account(ctx context.Context) (AccountView, error) {
	acc, err := s.account()
	if err != nil {
		return AccountView{}, err
	}
	var positions []model.Position
	if err := s.db.Where("account_id = ?", acc.ID).Order("id asc").Find(&positions).Error; err != nil {
		return AccountView{}, err
	}
	ids := make([]string, 0, len(positions))
	for _, pos := range positions {
		ids = append(ids, market.SecID(pos.Code, market.InferMarket(pos.Code)))
	}
	if len(ids) > 0 {
		_ = s.refreshQuotes(ctx, ids)
	}
	var snaps []model.QuoteSnap
	if len(ids) > 0 {
		s.db.Where("sec_id IN ?", ids).Find(&snaps)
	}
	priceByCode := map[string]float64{}
	for _, snap := range snaps {
		if snap.Kind == "index" {
			continue
		}
		priceByCode[snap.Code] = snap.Price
	}
	view := AccountView{
		Name: acc.Name, Cash: round2(acc.Cash), Initial: round2(acc.Initial),
		Positions: make([]PositionView, 0, len(positions)),
	}
	for _, pos := range positions {
		last := priceByCode[pos.Code]
		if last <= 0 {
			last = pos.AvgCost
		}
		mv := round2(float64(pos.Qty) * last)
		pnl := round2(mv - pos.CostBasis)
		pct := 0.0
		if pos.CostBasis > 0 {
			pct = round2(pnl / pos.CostBasis * 100)
		}
		view.MarketValue = round2(view.MarketValue + mv)
		view.Positions = append(view.Positions, PositionView{
			Position: pos, Last: last, MarketValue: mv, PnL: pnl, PnLPct: pct,
		})
	}
	view.Equity = round2(view.Cash + view.MarketValue)
	view.PnL = round2(view.Equity - view.Initial)
	if view.Initial > 0 {
		view.PnLPct = round2(view.PnL / view.Initial * 100)
	}
	return view, nil
}

func (s *Service) ResetAccount(ctx context.Context) (AccountView, error) {
	tradeMu.Lock()
	acc, err := s.account()
	if err != nil {
		tradeMu.Unlock()
		return AccountView{}, err
	}
	if err := s.db.Where("account_id = ?", acc.ID).Delete(&model.Position{}).Error; err != nil {
		tradeMu.Unlock()
		return AccountView{}, err
	}
	if err := s.db.Where("account_id = ?", acc.ID).Delete(&model.Order{}).Error; err != nil {
		tradeMu.Unlock()
		return AccountView{}, err
	}
	if err := s.db.Model(&model.Account{}).Where("id = ?", acc.ID).Updates(map[string]any{
		"cash": acc.Initial, "updated_at": time.Now(),
	}).Error; err != nil {
		tradeMu.Unlock()
		return AccountView{}, err
	}
	tradeMu.Unlock()
	return s.Account(ctx)
}

func (s *Service) Orders(limit int) ([]model.Order, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []model.Order
	err := s.db.Order("id desc").Limit(limit).Find(&rows).Error
	return rows, err
}

func (s *Service) PlaceOrder(ctx context.Context, in OrderInput) (model.Order, error) {
	if in.Side != "buy" && in.Side != "sell" {
		return model.Order{}, fmt.Errorf("方向只能是买入或卖出")
	}
	if in.Qty <= 0 || in.Qty%100 != 0 || in.Qty > 1_000_000 {
		return model.Order{}, fmt.Errorf("数量需为 100 股的整数倍")
	}
	ins, err := s.Resolve(ctx, in.Code)
	if err != nil {
		return model.Order{}, err
	}
	if err := s.refreshQuotes(ctx, []string{ins.SecID}); err != nil {
		var cached model.QuoteSnap
		if s.db.Where("sec_id = ?", ins.SecID).First(&cached).Error != nil || cached.Price <= 0 {
			return model.Order{}, err
		}
	}
	var snap model.QuoteSnap
	if err := s.db.Where("sec_id = ?", ins.SecID).First(&snap).Error; err != nil || snap.Price <= 0 {
		return model.Order{}, fmt.Errorf("没有可用的最新价")
	}
	if ins.Name == "" || ins.Name == ins.Code {
		ins.Name = snap.Name
	}
	source := in.Source
	if source == "" {
		source = "manual"
	}

	tradeMu.Lock()
	defer tradeMu.Unlock()

	acc, err := s.account()
	if err != nil {
		return model.Order{}, err
	}
	var pos model.Position
	s.db.Where("account_id = ? AND code = ?", acc.ID, ins.Code).First(&pos)
	amount := round2(float64(in.Qty) * snap.Price)
	fee := quant.Commission(in.Side, amount)
	order := model.Order{
		AccountID: acc.ID, Code: ins.Code, Name: ins.Name, Side: in.Side, Qty: in.Qty,
		Price: snap.Price, Amount: amount, Fee: fee, Status: "filled", Source: source, StrategyID: in.StrategyID,
	}
	if in.Side == "buy" {
		if amount+fee > acc.Cash+1e-6 {
			return s.reject(order, "现金不足")
		}
		cash := round2(acc.Cash - amount - fee)
		if pos.ID == 0 {
			pos = model.Position{
				AccountID: acc.ID, Code: ins.Code, Name: ins.Name, Qty: in.Qty,
				AvgCost: snap.Price, CostBasis: round2(amount + fee),
			}
			if err := s.db.Create(&pos).Error; err != nil {
				return model.Order{}, err
			}
		} else {
			pos.Qty += in.Qty
			pos.CostBasis = round2(pos.CostBasis + amount + fee)
			pos.AvgCost = round2(pos.CostBasis / float64(pos.Qty))
			pos.Name = ins.Name
			if err := s.db.Save(&pos).Error; err != nil {
				return model.Order{}, err
			}
		}
		if err := s.db.Model(&model.Account{}).Where("id = ?", acc.ID).Updates(map[string]any{
			"cash": cash, "updated_at": time.Now(),
		}).Error; err != nil {
			return model.Order{}, err
		}
		order.Message = "模拟买入成交"
	} else {
		if pos.ID == 0 || pos.Qty < in.Qty {
			return s.reject(order, "持仓不足")
		}
		released := pos.CostBasis * (float64(in.Qty) / float64(pos.Qty))
		pnl := round2(amount - fee - released)
		order.PnL = pnl
		cash := round2(acc.Cash + amount - fee)
		pos.Qty -= in.Qty
		pos.CostBasis = round2(pos.CostBasis - released)
		if pos.Qty == 0 {
			if err := s.db.Delete(&pos).Error; err != nil {
				return model.Order{}, err
			}
		} else {
			pos.AvgCost = round2(pos.CostBasis / float64(pos.Qty))
			if err := s.db.Save(&pos).Error; err != nil {
				return model.Order{}, err
			}
		}
		if err := s.db.Model(&model.Account{}).Where("id = ?", acc.ID).Updates(map[string]any{
			"cash": cash, "updated_at": time.Now(),
		}).Error; err != nil {
			return model.Order{}, err
		}
		order.Message = "模拟卖出成交"
	}
	if err := s.db.Create(&order).Error; err != nil {
		return model.Order{}, err
	}
	return order, nil
}

func (s *Service) reject(order model.Order, message string) (model.Order, error) {
	order.Status = "rejected"
	order.Message = message
	_ = s.db.Create(&order).Error
	return order, fmt.Errorf("%s", message)
}

func (s *Service) Strategies() ([]StrategyView, error) {
	var rows []model.Strategy
	if err := s.db.Order("id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]StrategyView, 0, len(rows))
	for _, row := range rows {
		out = append(out, strategyView(row))
	}
	return out, nil
}

func (s *Service) CreateStrategy(ctx context.Context, in StrategyInput) (StrategyView, error) {
	ins, err := s.Resolve(ctx, in.Code)
	if err != nil {
		return StrategyView{}, err
	}
	params := in.Params.Normalize(in.Type)
	if err := params.Validate(in.Type); err != nil {
		return StrategyView{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = strategyTitle(in.Type) + " · " + ins.Name
	}
	raw, _ := json.Marshal(params)
	acc, err := s.account()
	if err != nil {
		return StrategyView{}, err
	}
	row := model.Strategy{
		Name: name, Type: in.Type, Code: ins.Code, StockName: ins.Name,
		Params: string(raw), Enabled: in.Enabled, AccountID: acc.ID,
	}
	if err := s.db.Create(&row).Error; err != nil {
		return StrategyView{}, err
	}
	_ = s.syncKline(ctx, ins, 220)
	return strategyView(row), nil
}

func (s *Service) UpdateStrategy(ctx context.Context, id uint, patch StrategyPatch) (StrategyView, error) {
	row, err := s.strategy(id)
	if err != nil {
		return StrategyView{}, err
	}
	updates := map[string]any{"updated_at": time.Now()}
	if patch.Name != nil && strings.TrimSpace(*patch.Name) != "" {
		updates["name"] = strings.TrimSpace(*patch.Name)
	}
	if patch.Enabled != nil {
		updates["enabled"] = *patch.Enabled
	}
	if patch.Code != nil && strings.TrimSpace(*patch.Code) != "" {
		ins, err := s.Resolve(ctx, *patch.Code)
		if err != nil {
			return StrategyView{}, err
		}
		updates["code"] = ins.Code
		updates["stock_name"] = ins.Name
		row.Type = row.Type
	}
	kind := row.Type
	if patch.Params != nil {
		params := patch.Params.Normalize(kind)
		if err := params.Validate(kind); err != nil {
			return StrategyView{}, err
		}
		raw, _ := json.Marshal(params)
		updates["params"] = string(raw)
	}
	if err := s.db.Model(&model.Strategy{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return StrategyView{}, err
	}
	row, err = s.strategy(id)
	if err != nil {
		return StrategyView{}, err
	}
	return strategyView(row), nil
}

func (s *Service) DeleteStrategy(id uint) error {
	res := s.db.Delete(&model.Strategy{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("策略不存在")
	}
	return nil
}

func (s *Service) Backtest(ctx context.Context, id uint) (BacktestView, error) {
	row, err := s.strategy(id)
	if err != nil {
		return BacktestView{}, err
	}
	params := decodeParams(row.Params).Normalize(row.Type)
	view, err := s.Kline(ctx, row.Code, 500, true)
	if err != nil {
		return BacktestView{}, err
	}
	acc, err := s.account()
	if err != nil {
		return BacktestView{}, err
	}
	report, err := quant.Backtest(row.Type, toQuant(view.Bars), acc.Initial, params)
	if err != nil {
		return BacktestView{}, err
	}
	detail, _ := json.Marshal(struct {
		Curve []quant.Point `json:"curve"`
		Fills []quant.Fill  `json:"fills"`
	}{Curve: report.Curve, Fills: report.Fills})
	saved := model.Backtest{
		StrategyID: row.ID, Code: row.Code, Type: row.Type, Initial: report.Initial,
		FinalEquity: report.FinalEquity, ReturnPct: report.ReturnPct, MaxDrawdown: report.MaxDrawdown,
		Trades: report.Trades, WinRate: report.WinRate, Detail: string(detail),
	}
	if err := s.db.Create(&saved).Error; err != nil {
		return BacktestView{}, err
	}
	return backtestView(saved), nil
}

func (s *Service) Backtests(strategyID uint, limit int) ([]BacktestView, error) {
	if limit <= 0 || limit > 20 {
		limit = 8
	}
	q := s.db.Order("id desc").Limit(limit)
	if strategyID > 0 {
		q = q.Where("strategy_id = ?", strategyID)
	}
	var rows []model.Backtest
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]BacktestView, 0, len(rows))
	for _, row := range rows {
		out = append(out, backtestView(row))
	}
	return out, nil
}

func (s *Service) RunStrategy(ctx context.Context, id uint) (RunResult, error) {
	row, err := s.strategy(id)
	if err != nil {
		return RunResult{}, err
	}
	return s.runRow(ctx, row)
}

func (s *Service) RunEnabled(ctx context.Context) ([]RunResult, error) {
	var rows []model.Strategy
	if err := s.db.Where("enabled = ?", true).Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]RunResult, 0, len(rows))
	for _, row := range rows {
		result, err := s.runRow(ctx, row)
		if err != nil {
			result.Message = err.Error()
			if result.StrategyID == 0 {
				result.StrategyID = row.ID
				result.Name = row.Name
				result.Code = row.Code
			}
		}
		out = append(out, result)
	}
	return out, nil
}

func (s *Service) runRow(ctx context.Context, row model.Strategy) (RunResult, error) {
	params := decodeParams(row.Params).Normalize(row.Type)
	view, err := s.Kline(ctx, row.Code, 220, true)
	if err != nil {
		return RunResult{StrategyID: row.ID, Name: row.Name, Code: row.Code}, err
	}
	bars := toQuant(view.Bars)
	desired, err := quant.Latest(row.Type, bars, params)
	if err != nil {
		return RunResult{StrategyID: row.ID, Name: row.Name, Code: row.Code}, err
	}
	signal := "flat"
	if desired == 1 {
		signal = "long"
	}
	var pos model.Position
	s.db.Where("code = ?", row.Code).First(&pos)
	result := RunResult{StrategyID: row.ID, Name: row.Name, Code: row.Code, Signal: signal}
	switch {
	case desired == 1 && pos.Qty > 0:
		result.Message = "信号为持有，已有仓位"
	case desired == 0 && pos.Qty == 0:
		result.Message = "信号为空仓，未下单"
	case desired == 1 && pos.Qty == 0:
		account, err := s.Account(ctx)
		if err != nil {
			return result, err
		}
		price := bars[len(bars)-1].Close
		qty := quant.LotSize(account.Equity*params.PositionPct, price, account.Cash)
		if qty <= 0 {
			result.Message = "现金不足，无法买入 1 手"
			break
		}
		order, err := s.PlaceOrder(ctx, OrderInput{
			Code: row.Code, Side: "buy", Qty: qty, Source: "strategy", StrategyID: row.ID,
		})
		if err != nil {
			result.Message = err.Error()
			break
		}
		result.Acted = true
		result.Order = &order
		result.Message = fmt.Sprintf("买入 %d 股，模拟成交价 %.2f", order.Qty, order.Price)
	case desired == 0 && pos.Qty > 0:
		qty := pos.Qty / 100 * 100
		if qty <= 0 {
			result.Message = "持仓不足 1 手，未卖出"
			break
		}
		order, err := s.PlaceOrder(ctx, OrderInput{
			Code: row.Code, Side: "sell", Qty: qty, Source: "strategy", StrategyID: row.ID,
		})
		if err != nil {
			result.Message = err.Error()
			break
		}
		result.Acted = true
		result.Order = &order
		result.Message = fmt.Sprintf("卖出 %d 股，实现盈亏 %.2f", order.Qty, order.PnL)
	}
	now := time.Now()
	_ = s.db.Model(&model.Strategy{}).Where("id = ?", row.ID).Updates(map[string]any{
		"last_signal": signal, "last_message": result.Message, "last_run_at": now, "updated_at": now,
	}).Error
	return result, nil
}

func (s *Service) strategy(id uint) (model.Strategy, error) {
	var row model.Strategy
	if err := s.db.First(&row, id).Error; err != nil {
		return row, fmt.Errorf("策略不存在")
	}
	return row, nil
}

func strategyView(row model.Strategy) StrategyView {
	return StrategyView{Strategy: row, ParamSet: decodeParams(row.Params)}
}

func decodeParams(raw string) quant.Params {
	var params quant.Params
	_ = json.Unmarshal([]byte(raw), &params)
	return params
}

func backtestView(row model.Backtest) BacktestView {
	var detail struct {
		Curve []quant.Point `json:"curve"`
		Fills []quant.Fill  `json:"fills"`
	}
	_ = json.Unmarshal([]byte(row.Detail), &detail)
	fills := detail.Fills
	if len(fills) > 40 {
		fills = fills[len(fills)-40:]
	}
	return BacktestView{Backtest: row, Curve: detail.Curve, Fills: fills, Note: backtestNote}
}

func strategyTitle(kind string) string {
	switch kind {
	case "ma_cross":
		return "双均线"
	case "rsi":
		return "RSI 回归"
	case "momentum":
		return "动量突破"
	default:
		return "策略"
	}
}

func toQuant(rows []model.DailyBar) []quant.Bar {
	out := make([]quant.Bar, len(rows))
	for i, row := range rows {
		out[i] = quant.Bar{
			Date: row.Date, Open: row.Open, High: row.High, Low: row.Low, Close: row.Close, Volume: row.Volume,
		}
	}
	return out
}
