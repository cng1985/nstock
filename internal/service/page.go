package service

import (
	"nstock/internal/model"
)

func pageBounds(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 200 {
		size = 200
	}
	return page, size
}

func (s *Service) PageOrders(code string, page, size int, asc bool) ([]model.Order, int64, error) {
	page, size = pageBounds(page, size)
	countQ := s.db.Model(&model.Order{})
	findQ := s.db.Model(&model.Order{})
	if code != "" {
		countQ = countQ.Where("code = ?", code)
		findQ = findQ.Where("code = ?", code)
	}
	var total int64
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	order := "id desc"
	if asc {
		order = "id asc"
	}
	var rows []model.Order
	err := findQ.Order(order).Offset((page - 1) * size).Limit(size).Find(&rows).Error
	return rows, total, err
}

func (s *Service) PagePredictions(code string, page, size int, asc bool) ([]PredictionView, int64, error) {
	page, size = pageBounds(page, size)
	countQ := s.db.Model(&model.Prediction{})
	findQ := s.db.Model(&model.Prediction{})
	if code != "" {
		countQ = countQ.Where("code = ?", code)
		findQ = findQ.Where("code = ?", code)
	}
	var total int64
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	order := "id desc"
	if asc {
		order = "id asc"
	}
	var rows []model.Prediction
	if err := findQ.Order(order).Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return predictionViews(rows), total, nil
}

func (s *Service) PageEvents(code string, page, size int, asc bool) ([]model.AlertEvent, int64, error) {
	page, size = pageBounds(page, size)
	countQ := s.db.Model(&model.AlertEvent{})
	findQ := s.db.Model(&model.AlertEvent{})
	if code != "" {
		countQ = countQ.Where("code = ?", code)
		findQ = findQ.Where("code = ?", code)
	}
	var total int64
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	order := "id desc"
	if asc {
		order = "id asc"
	}
	var rows []model.AlertEvent
	err := findQ.Order(order).Offset((page - 1) * size).Limit(size).Find(&rows).Error
	return rows, total, err
}

func (s *Service) PageBacktests(strategyID uint, page, size int, asc bool) ([]BacktestView, int64, error) {
	page, size = pageBounds(page, size)
	countQ := s.db.Model(&model.Backtest{})
	findQ := s.db.Model(&model.Backtest{})
	if strategyID > 0 {
		countQ = countQ.Where("strategy_id = ?", strategyID)
		findQ = findQ.Where("strategy_id = ?", strategyID)
	}
	var total int64
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	order := "id desc"
	if asc {
		order = "id asc"
	}
	var rows []model.Backtest
	if err := findQ.Order(order).Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]BacktestView, 0, len(rows))
	for _, row := range rows {
		out = append(out, backtestView(row))
	}
	return out, total, nil
}
