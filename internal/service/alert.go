package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"nstock/internal/model"
)

type ScanResult struct {
	Checked   int       `json:"checked"`
	Fired     int       `json:"fired"`
	ScannedAt time.Time `json:"scannedAt"`
}

type RuleInput struct {
	Code      string
	Kind      string
	Threshold float64
	Name      string
}

type RulePatch struct {
	Name      *string
	Enabled   *bool
	Threshold *float64
}

func (s *Service) Rules() ([]model.AlertRule, error) {
	var rows []model.AlertRule
	err := s.db.Order("id desc").Find(&rows).Error
	return rows, err
}

func (s *Service) CreateRule(ctx context.Context, in RuleInput) (model.AlertRule, error) {
	switch in.Kind {
	case "price_above", "price_below", "pct_above", "pct_below":
	default:
		return model.AlertRule{}, fmt.Errorf("不支持的盯盘条件")
	}
	if in.Threshold == 0 || math.IsNaN(in.Threshold) {
		return model.AlertRule{}, fmt.Errorf("请填写有效阈值")
	}
	ins, err := s.Resolve(ctx, in.Code)
	if err != nil {
		return model.AlertRule{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = RuleTitle(in.Kind, in.Threshold)
	}
	row := model.AlertRule{
		Code: ins.Code, SecID: ins.SecID, Name: name, StockName: ins.Name,
		Kind: in.Kind, Threshold: in.Threshold, Enabled: true,
	}
	if err := s.db.Create(&row).Error; err != nil {
		return model.AlertRule{}, err
	}
	return row, nil
}

func (s *Service) UpdateRule(id uint, patch RulePatch) (model.AlertRule, error) {
	var row model.AlertRule
	if err := s.db.First(&row, id).Error; err != nil {
		return row, fmt.Errorf("盯盘条件不存在")
	}
	updates := map[string]any{"updated_at": time.Now()}
	if patch.Name != nil && strings.TrimSpace(*patch.Name) != "" {
		updates["name"] = strings.TrimSpace(*patch.Name)
	}
	if patch.Enabled != nil {
		updates["enabled"] = *patch.Enabled
	}
	if patch.Threshold != nil {
		if *patch.Threshold == 0 {
			return row, fmt.Errorf("请填写有效阈值")
		}
		updates["threshold"] = *patch.Threshold
		updates["latched"] = false
	}
	if err := s.db.Model(&model.AlertRule{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return row, err
	}
	if err := s.db.First(&row, id).Error; err != nil {
		return row, err
	}
	return row, nil
}

func (s *Service) DeleteRule(id uint) error {
	res := s.db.Delete(&model.AlertRule{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("盯盘条件不存在")
	}
	return nil
}

func (s *Service) Events(limit int) ([]model.AlertEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []model.AlertEvent
	err := s.db.Order("id desc").Limit(limit).Find(&rows).Error
	return rows, err
}

func (s *Service) Scan(ctx context.Context) (ScanResult, error) {
	var rules []model.AlertRule
	if err := s.db.Where("enabled = ?", true).Find(&rules).Error; err != nil {
		return ScanResult{}, err
	}
	var watches []model.WatchItem
	if err := s.db.Find(&watches).Error; err != nil {
		return ScanResult{}, err
	}
	ids := make([]string, 0, len(watches)+len(rules))
	for _, item := range watches {
		ids = append(ids, item.SecID)
	}
	for _, rule := range rules {
		ids = append(ids, rule.SecID)
	}
	ids = uniq(ids)
	now := time.Now()
	if len(ids) == 0 {
		return ScanResult{ScannedAt: now}, nil
	}
	if err := s.refreshQuotes(ctx, ids); err != nil {
		return ScanResult{}, err
	}
	var snaps []model.QuoteSnap
	if err := s.db.Where("sec_id IN ?", ids).Find(&snaps).Error; err != nil {
		return ScanResult{}, err
	}
	bySec := make(map[string]model.QuoteSnap, len(snaps))
	for _, snap := range snaps {
		bySec[snap.SecID] = snap
	}
	fired := 0
	for _, rule := range rules {
		snap, ok := bySec[rule.SecID]
		if !ok || snap.Price <= 0 {
			continue
		}
		hit := ruleHit(rule, snap)
		switch {
		case hit && !rule.Latched:
			name := snap.Name
			if name == "" {
				name = rule.StockName
			}
			msg := fmt.Sprintf("%s %s 现价 %.2f，触发「%s」", name, rule.Code, snap.Price, rule.Name)
			if err := s.db.Create(&model.AlertEvent{
				RuleID: rule.ID, Code: rule.Code, StockName: name, Message: msg, Price: snap.Price,
			}).Error; err != nil {
				return ScanResult{}, err
			}
			fired++
			if err := s.touchRule(rule.ID, true, snap.Price); err != nil {
				return ScanResult{}, err
			}
		case !hit && rule.Latched:
			if err := s.touchRule(rule.ID, false, snap.Price); err != nil {
				return ScanResult{}, err
			}
		default:
			if err := s.db.Model(&model.AlertRule{}).Where("id = ?", rule.ID).Updates(map[string]any{
				"last_price": snap.Price, "updated_at": time.Now(),
			}).Error; err != nil {
				return ScanResult{}, err
			}
		}
	}
	return ScanResult{Checked: len(rules), Fired: fired, ScannedAt: time.Now()}, nil
}

func (s *Service) touchRule(id uint, latched bool, price float64) error {
	return s.db.Model(&model.AlertRule{}).Where("id = ?", id).Updates(map[string]any{
		"latched": latched, "last_price": price, "updated_at": time.Now(),
	}).Error
}

func ruleHit(rule model.AlertRule, snap model.QuoteSnap) bool {
	switch rule.Kind {
	case "price_above":
		return snap.Price >= rule.Threshold
	case "price_below":
		return snap.Price <= rule.Threshold
	case "pct_above":
		return snap.ChangePct >= rule.Threshold
	case "pct_below":
		limit := rule.Threshold
		if limit > 0 {
			limit = -limit
		}
		return snap.ChangePct <= limit
	default:
		return false
	}
}

func RuleTitle(kind string, threshold float64) string {
	switch kind {
	case "price_above":
		return fmt.Sprintf("价格高于 %.2f", threshold)
	case "price_below":
		return fmt.Sprintf("价格低于 %.2f", threshold)
	case "pct_above":
		return fmt.Sprintf("涨幅达到 %.2f%%", threshold)
	case "pct_below":
		return fmt.Sprintf("跌幅达到 %.2f%%", math.Abs(threshold))
	default:
		return "盯盘"
	}
}
