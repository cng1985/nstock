package service

import (
	"errors"
	"math"

	"nstock/internal/market"
	"nstock/internal/model"

	"gorm.io/gorm"
)

var (
	ErrNotFound = errors.New("未找到证券")
	ErrUpstream = errors.New("行情源")
)

type Service struct {
	db  *gorm.DB
	src market.Provider
}

func New(db *gorm.DB, src market.Provider) *Service {
	return &Service{db: db, src: src}
}

func Seed(db *gorm.DB) error {
	var n int64
	if err := db.Model(&model.Account{}).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		if err := db.Create(&model.Account{Name: "模拟账户", Cash: 1_000_000, Initial: 1_000_000}).Error; err != nil {
			return err
		}
	}
	if err := db.Model(&model.WatchItem{}).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	seeds := []model.WatchItem{
		{Code: "600519", Name: "贵州茅台", Market: "sh", SecID: "1.600519"},
		{Code: "000001", Name: "平安银行", Market: "sz", SecID: "0.000001"},
		{Code: "300750", Name: "宁德时代", Market: "sz", SecID: "0.300750"},
		{Code: "601318", Name: "中国平安", Market: "sh", SecID: "1.601318"},
		{Code: "000858", Name: "五粮液", Market: "sz", SecID: "0.000858"},
		{Code: "002594", Name: "比亚迪", Market: "sz", SecID: "0.002594"},
	}
	return db.Create(&seeds).Error
}

func (s *Service) account() (model.Account, error) {
	var acc model.Account
	if err := s.db.Order("id asc").First(&acc).Error; err != nil {
		return acc, errors.New("模拟账户未初始化")
	}
	return acc, nil
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func uniq(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
