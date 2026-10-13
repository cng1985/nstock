package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"nstock/internal/market"
	"nstock/internal/model"
	"nstock/internal/predict"

	"gorm.io/gorm/clause"
)

type Instrument struct {
	Code   string
	Name   string
	Market string
	SecID  string
}

type WatchQuote struct {
	model.WatchItem
	Quote *model.QuoteSnap `json:"quote"`
}

type WatchList struct {
	Items   []WatchQuote `json:"items"`
	Warning string       `json:"warning,omitempty"`
}

type KlineView struct {
	Code   string           `json:"code"`
	Name   string           `json:"name"`
	Market string           `json:"market"`
	Bars   []model.DailyBar `json:"bars"`
}

type PredictionView struct {
	model.Prediction
	ReasonItems []predict.Reason `json:"reasons"`
}

var indexMeta = []struct {
	SecID string
	Name  string
}{
	{"1.000001", "上证指数"},
	{"0.399001", "深证成指"},
	{"0.399006", "创业板指"},
	{"1.000300", "沪深300"},
}

func indexName(secID string) string {
	for _, item := range indexMeta {
		if item.SecID == secID {
			return item.Name
		}
	}
	return ""
}

func isIndex(secID string) bool {
	return indexName(secID) != ""
}

func (s *Service) Warm(ctx context.Context) {
	c, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if _, err := s.Indices(c); err != nil {
		slog.Warn("指数行情暂不可用", "err", err)
	}
	if _, err := s.WatchQuotes(c, true); err != nil {
		slog.Warn("自选行情暂不可用", "err", err)
	}
	var items []model.WatchItem
	if err := s.db.Find(&items).Error; err != nil {
		return
	}
	for _, it := range items {
		if c.Err() != nil {
			return
		}
		err := s.syncKline(c, Instrument{Code: it.Code, Name: it.Name, Market: it.Market, SecID: it.SecID}, 220)
		if err != nil {
			slog.Warn("K线暂不可用", "code", it.Code, "err", err)
		}
	}
}

func (s *Service) Search(ctx context.Context, query string) ([]market.Hit, error) {
	hits, err := s.src.Search(ctx, query)
	if err != nil {
		if code, nerr := market.NormalizeCode(query); nerr == nil {
			mkt := market.InferMarket(code)
			return []market.Hit{{
				Code: code, Name: code, Market: mkt, SecID: market.SecID(code, mkt), KindName: "代码",
			}}, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	out := make([]market.Hit, 0, len(hits))
	for _, hit := range hits {
		if strings.Contains(hit.KindName, "指") {
			continue
		}
		out = append(out, hit)
	}
	if len(out) == 0 {
		if code, nerr := market.NormalizeCode(query); nerr == nil {
			mkt := market.InferMarket(code)
			out = append(out, market.Hit{Code: code, Name: code, Market: mkt, SecID: market.SecID(code, mkt), KindName: "代码"})
		}
	}
	return out, nil
}

func (s *Service) Resolve(ctx context.Context, raw string) (Instrument, error) {
	if code, err := market.NormalizeCode(raw); err == nil {
		var item model.WatchItem
		if err := s.db.Where("code = ?", code).First(&item).Error; err == nil {
			return Instrument{Code: item.Code, Name: item.Name, Market: item.Market, SecID: item.SecID}, nil
		}
		hits, serr := s.src.Search(ctx, code)
		if serr == nil {
			for _, hit := range hits {
				if hit.Code == code && !strings.Contains(hit.KindName, "指") {
					name := hit.Name
					if name == "" {
						name = code
					}
					return Instrument{Code: hit.Code, Name: name, Market: hit.Market, SecID: hit.SecID}, nil
				}
			}
		}
		mkt := market.InferMarket(code)
		secID := market.SecID(code, mkt)
		k, kerr := s.src.Kline(ctx, secID, 5)
		if kerr != nil {
			if serr != nil {
				return Instrument{}, fmt.Errorf("%w: %v", ErrUpstream, serr)
			}
			return Instrument{}, ErrNotFound
		}
		name := k.Name
		if name == "" {
			name = code
		}
		return Instrument{Code: code, Name: name, Market: mkt, SecID: secID}, nil
	}
	hits, err := s.src.Search(ctx, strings.TrimSpace(raw))
	if err != nil {
		return Instrument{}, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	for _, hit := range hits {
		if strings.Contains(hit.KindName, "指") {
			continue
		}
		return Instrument{Code: hit.Code, Name: hit.Name, Market: hit.Market, SecID: hit.SecID}, nil
	}
	return Instrument{}, ErrNotFound
}

func (s *Service) ListWatch() ([]model.WatchItem, error) {
	var items []model.WatchItem
	err := s.db.Order("id asc").Find(&items).Error
	return items, err
}

func (s *Service) AddWatch(ctx context.Context, raw string) (model.WatchItem, error) {
	ins, err := s.Resolve(ctx, raw)
	if err != nil {
		return model.WatchItem{}, err
	}
	var existing model.WatchItem
	if err := s.db.Where("code = ?", ins.Code).First(&existing).Error; err == nil {
		return existing, nil
	}
	item := model.WatchItem{Code: ins.Code, Name: ins.Name, Market: ins.Market, SecID: ins.SecID}
	if err := s.db.Create(&item).Error; err != nil {
		return model.WatchItem{}, err
	}
	_ = s.refreshQuotes(ctx, []string{ins.SecID})
	_ = s.syncKline(ctx, ins, 180)
	return item, nil
}

func (s *Service) RemoveWatch(raw string) error {
	code, err := market.NormalizeCode(raw)
	if err != nil {
		return err
	}
	res := s.db.Where("code = ?", code).Delete(&model.WatchItem{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("不在自选列表中")
	}
	return nil
}

func (s *Service) WatchQuotes(ctx context.Context, refresh bool) (WatchList, error) {
	items, err := s.ListWatch()
	if err != nil {
		return WatchList{}, err
	}
	out := WatchList{Items: make([]WatchQuote, 0, len(items))}
	if len(items) == 0 {
		return out, nil
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.SecID)
	}
	if refresh {
		if err := s.refreshQuotes(ctx, ids); err != nil {
			out.Warning = err.Error()
		}
	}
	var snaps []model.QuoteSnap
	if err := s.db.Where("sec_id IN ?", ids).Find(&snaps).Error; err != nil {
		return WatchList{}, err
	}
	by := make(map[string]model.QuoteSnap, len(snaps))
	for _, snap := range snaps {
		by[snap.SecID] = snap
	}
	for _, it := range items {
		row := WatchQuote{WatchItem: it}
		if snap, ok := by[it.SecID]; ok {
			copy := snap
			row.Quote = &copy
		}
		out.Items = append(out.Items, row)
	}
	return out, nil
}

func (s *Service) Indices(ctx context.Context) ([]model.QuoteSnap, error) {
	ids := make([]string, 0, len(indexMeta))
	for _, item := range indexMeta {
		ids = append(ids, item.SecID)
	}
	err := s.refreshQuotes(ctx, ids)
	var rows []model.QuoteSnap
	if qerr := s.db.Where("sec_id IN ?", ids).Find(&rows).Error; qerr != nil {
		return nil, qerr
	}
	by := make(map[string]model.QuoteSnap, len(rows))
	for _, row := range rows {
		by[row.SecID] = row
	}
	out := make([]model.QuoteSnap, 0, len(ids))
	for _, id := range ids {
		row, ok := by[id]
		if !ok {
			continue
		}
		if row.Name == "" {
			row.Name = indexName(id)
		}
		out = append(out, row)
	}
	if len(out) == 0 && err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) Quote(ctx context.Context, raw string, refresh bool) (model.QuoteSnap, error) {
	ins, err := s.Resolve(ctx, raw)
	if err != nil {
		return model.QuoteSnap{}, err
	}
	if refresh {
		if err := s.refreshQuotes(ctx, []string{ins.SecID}); err != nil {
			var snap model.QuoteSnap
			if s.db.Where("sec_id = ?", ins.SecID).First(&snap).Error == nil && snap.Price > 0 {
				return snap, nil
			}
			return model.QuoteSnap{}, err
		}
	}
	var snap model.QuoteSnap
	if err := s.db.Where("sec_id = ?", ins.SecID).First(&snap).Error; err != nil {
		if !refresh {
			return s.Quote(ctx, ins.Code, true)
		}
		return model.QuoteSnap{}, ErrNotFound
	}
	if snap.Name == "" {
		snap.Name = ins.Name
	}
	return snap, nil
}

func (s *Service) Kline(ctx context.Context, raw string, limit int, refresh bool) (KlineView, error) {
	if limit <= 0 {
		limit = 160
	}
	if limit > 800 {
		limit = 800
	}
	ins, err := s.Resolve(ctx, raw)
	if err != nil {
		return KlineView{}, err
	}
	if refresh {
		if err := s.syncKline(ctx, ins, limit); err != nil {
			var count int64
			s.db.Model(&model.DailyBar{}).Where("code = ?", ins.Code).Count(&count)
			if count == 0 {
				return KlineView{}, err
			}
		}
	}
	var bars []model.DailyBar
	if err := s.db.Where("code = ?", ins.Code).Order("date asc").Find(&bars).Error; err != nil {
		return KlineView{}, err
	}
	if len(bars) > limit {
		bars = bars[len(bars)-limit:]
	}
	name := ins.Name
	if name == "" && len(bars) > 0 {
		name = ins.Code
	}
	return KlineView{Code: ins.Code, Name: name, Market: ins.Market, Bars: bars}, nil
}

func (s *Service) refreshQuotes(ctx context.Context, secIDs []string) error {
	ids := uniq(secIDs)
	if len(ids) == 0 {
		return nil
	}
	now := time.Now()
	for i := 0; i < len(ids); i += 40 {
		end := i + 40
		if end > len(ids) {
			end = len(ids)
		}
		quotes, err := s.src.Quotes(ctx, ids[i:end])
		if err != nil {
			return fmt.Errorf("%w: %v", ErrUpstream, err)
		}
		for _, q := range quotes {
			if q.SecID == "" || q.Price <= 0 {
				continue
			}
			kind := "stock"
			if isIndex(q.SecID) {
				kind = "index"
			}
			name := q.Name
			if name == "" {
				name = indexName(q.SecID)
			}
			snap := model.QuoteSnap{
				SecID: q.SecID, Code: q.Code, Name: name, Market: q.Market, Kind: kind,
				Price: q.Price, Change: q.Change, ChangePct: q.ChangePct,
				Open: q.Open, High: q.High, Low: q.Low, PreClose: q.PreClose,
				Volume: q.Volume, Amount: q.Amount, UpdatedAt: now,
			}
			if err := s.db.Save(&snap).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) syncKline(ctx context.Context, ins Instrument, limit int) error {
	if limit < 180 {
		limit = 180
	}
	k, err := s.src.Kline(ctx, ins.SecID, limit)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	if k.Name != "" && (ins.Name == "" || ins.Name == ins.Code) {
		s.db.Model(&model.WatchItem{}).Where("code = ? AND (name = '' OR name = code)", ins.Code).Update("name", k.Name)
	}
	rows := make([]model.DailyBar, 0, len(k.Bars))
	for _, bar := range k.Bars {
		if bar.Date == "" || bar.Close <= 0 {
			continue
		}
		rows = append(rows, model.DailyBar{
			Code: ins.Code, Date: bar.Date, Open: bar.Open, High: bar.High,
			Low: bar.Low, Close: bar.Close, Volume: bar.Volume, Amount: bar.Amount,
		})
	}
	if len(rows) == 0 {
		return fmt.Errorf("%w: 没有可用K线", ErrUpstream)
	}
	for i := 0; i < len(rows); i += 80 {
		end := i + 80
		if end > len(rows) {
			end = len(rows)
		}
		err := s.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "code"}, {Name: "date"}},
			DoUpdates: clause.AssignmentColumns([]string{"open", "high", "low", "close", "volume", "amount"}),
		}).Create(rows[i:end]).Error
		if err != nil {
			return err
		}
	}
	return nil
}
