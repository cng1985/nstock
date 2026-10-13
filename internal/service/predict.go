package service

import (
	"context"
	"encoding/json"

	"nstock/internal/model"
	"nstock/internal/predict"
)

func (s *Service) Predict(ctx context.Context, raw string) (PredictionView, error) {
	view, err := s.Kline(ctx, raw, 220, true)
	if err != nil {
		return PredictionView{}, err
	}
	result, err := predict.Run(toPredict(view.Bars))
	if err != nil {
		return PredictionView{}, err
	}
	encoded, _ := json.Marshal(result.Reasons)
	row := model.Prediction{
		Code: view.Code, Name: view.Name, Direction: result.Direction,
		Confidence: result.Confidence, Score: result.Score, Horizon: "1d",
		Reasons: string(encoded), Close: result.Close,
	}
	if err := s.db.Create(&row).Error; err != nil {
		return PredictionView{}, err
	}
	return PredictionView{Prediction: row, ReasonItems: result.Reasons}, nil
}

func (s *Service) Predictions(code string, limit int) ([]PredictionView, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q := s.db.Order("id desc").Limit(limit)
	if code != "" {
		q = q.Where("code = ?", code)
	}
	var rows []model.Prediction
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]PredictionView, 0, len(rows))
	for _, row := range rows {
		var reasons []predict.Reason
		_ = json.Unmarshal([]byte(row.Reasons), &reasons)
		out = append(out, PredictionView{Prediction: row, ReasonItems: reasons})
	}
	return out, nil
}

func toPredict(rows []model.DailyBar) []predict.Bar {
	out := make([]predict.Bar, len(rows))
	for i, row := range rows {
		out[i] = predict.Bar{
			Date: row.Date, Open: row.Open, High: row.High, Low: row.Low, Close: row.Close, Volume: row.Volume,
		}
	}
	return out
}
