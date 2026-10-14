package model

import "time"

type WatchItem struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Code      string    `json:"code" gorm:"uniqueIndex;size:16"`
	Name      string    `json:"name"`
	Market    string    `json:"market" gorm:"size:8"`
	SecID     string    `json:"secId" gorm:"uniqueIndex;size:16"`
	CreatedAt time.Time `json:"createdAt"`
}

type QuoteSnap struct {
	SecID     string    `json:"secId" gorm:"primaryKey;size:16"`
	Code      string    `json:"code" gorm:"index;size:16"`
	Name      string    `json:"name"`
	Market    string    `json:"market" gorm:"size:8"`
	Kind      string    `json:"kind" gorm:"size:16"`
	Price     float64   `json:"price"`
	Change    float64   `json:"change"`
	ChangePct float64   `json:"changePct"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	PreClose  float64   `json:"preClose"`
	Volume    float64   `json:"volume"`
	Amount    float64   `json:"amount"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type DailyBar struct {
	ID     uint    `json:"id" gorm:"primaryKey"`
	Code   string  `json:"code" gorm:"uniqueIndex:idx_code_date;size:16"`
	Date   string  `json:"date" gorm:"uniqueIndex:idx_code_date;size:16"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
	Amount float64 `json:"amount"`
}

type Prediction struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Code       string    `json:"code" gorm:"index;size:16"`
	Name       string    `json:"name"`
	Direction  string    `json:"direction" gorm:"size:8"`
	Confidence float64   `json:"confidence"`
	Score      float64   `json:"score"`
	Horizon    string    `json:"horizon" gorm:"size:8"`
	Reasons    string    `json:"-" gorm:"type:text"`
	Close      float64   `json:"close"`
	CreatedAt  time.Time `json:"createdAt"`
}

type Account struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name"`
	Cash      float64   `json:"cash"`
	Initial   float64   `json:"initial"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Position struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	AccountID uint      `json:"accountId" gorm:"uniqueIndex:idx_pos"`
	Code      string    `json:"code" gorm:"uniqueIndex:idx_pos;size:16"`
	Name      string    `json:"name"`
	Qty       int       `json:"qty"`
	AvgCost   float64   `json:"avgCost"`
	CostBasis float64   `json:"costBasis"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Order struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	AccountID  uint      `json:"accountId" gorm:"index"`
	Code       string    `json:"code" gorm:"index;size:16"`
	Name       string    `json:"name"`
	Side       string    `json:"side" gorm:"size:8"`
	Qty        int       `json:"qty"`
	Price      float64   `json:"price"`
	Amount     float64   `json:"amount"`
	Fee        float64   `json:"fee"`
	PnL        float64   `json:"pnl"`
	Status     string    `json:"status" gorm:"size:16"`
	Source     string    `json:"source" gorm:"size:16"`
	StrategyID uint      `json:"strategyId"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"createdAt"`
}

type Strategy struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	Name        string     `json:"name"`
	Type        string     `json:"type" gorm:"size:32"`
	Code        string     `json:"code" gorm:"size:16;index"`
	StockName   string     `json:"stockName"`
	Params      string     `json:"-" gorm:"type:text"`
	Enabled     bool       `json:"enabled"`
	AccountID   uint       `json:"accountId"`
	LastSignal  string     `json:"lastSignal" gorm:"size:16"`
	LastMessage string     `json:"lastMessage"`
	LastRunAt   *time.Time `json:"lastRunAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type Backtest struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	StrategyID  uint      `json:"strategyId" gorm:"index"`
	Code        string    `json:"code" gorm:"size:16"`
	Type        string    `json:"type" gorm:"size:32"`
	Initial     float64   `json:"initial"`
	FinalEquity float64   `json:"finalEquity"`
	ReturnPct   float64   `json:"returnPct"`
	MaxDrawdown float64   `json:"maxDrawdown"`
	Trades      int       `json:"trades"`
	WinRate     float64   `json:"winRate"`
	Detail      string    `json:"-" gorm:"type:text"`
	CreatedAt   time.Time `json:"createdAt"`
}

type AlertRule struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Code      string    `json:"code" gorm:"index;size:16"`
	SecID     string    `json:"secId" gorm:"size:16"`
	Name      string    `json:"name"`
	StockName string    `json:"stockName"`
	Kind      string    `json:"kind" gorm:"size:32"`
	Threshold float64   `json:"threshold"`
	Enabled   bool      `json:"enabled"`
	Latched   bool      `json:"latched"`
	LastPrice float64   `json:"lastPrice"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type AlertEvent struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	RuleID    uint      `json:"ruleId" gorm:"index"`
	Code      string    `json:"code" gorm:"size:16;index"`
	StockName string    `json:"stockName"`
	Message   string    `json:"message"`
	Price     float64   `json:"price"`
	CreatedAt time.Time `json:"createdAt"`
}

type User struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	Username     string    `json:"username" gorm:"uniqueIndex;size:64"`
	DisplayName  string    `json:"displayName" gorm:"size:64"`
	PasswordHash string    `json:"-" gorm:"size:128"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Role struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Code        string    `json:"code" gorm:"uniqueIndex;size:64"`
	Name        string    `json:"name" gorm:"size:64"`
	Description string    `json:"description" gorm:"size:200"`
	BuiltIn     bool      `json:"builtIn"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Permission struct {
	Code      string `json:"code" gorm:"primaryKey;size:64"`
	Name      string `json:"name"`
	GroupName string `json:"group" gorm:"column:group_name;size:32"`
}

type UserRole struct {
	UserID uint `gorm:"primaryKey"`
	RoleID uint `gorm:"primaryKey"`
}

type RolePermission struct {
	RoleID uint   `gorm:"primaryKey"`
	Code   string `gorm:"primaryKey;size:64"`
}

type Session struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	TokenHash string    `json:"-" gorm:"uniqueIndex;size:64"`
	UserID    uint      `json:"userId" gorm:"index"`
	ExpiresAt time.Time `json:"expiresAt" gorm:"index"`
	CreatedAt time.Time `json:"createdAt"`
}
