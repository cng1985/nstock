package quant

import (
	"fmt"
	"math"
)

type Bar struct {
	Date   string
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

type Params struct {
	Fast        int     `json:"fast"`
	Slow        int     `json:"slow"`
	RSIPeriod   int     `json:"rsiPeriod"`
	Oversold    float64 `json:"oversold"`
	Overbought  float64 `json:"overbought"`
	Lookback    int     `json:"lookback"`
	Threshold   float64 `json:"threshold"`
	MAPeriod    int     `json:"maPeriod"`
	PositionPct float64 `json:"positionPct"`
}

type Point struct {
	Date   string  `json:"date"`
	Equity float64 `json:"equity"`
}

type Fill struct {
	Date  string  `json:"date"`
	Side  string  `json:"side"`
	Price float64 `json:"price"`
	Qty   int     `json:"qty"`
	Fee   float64 `json:"fee"`
	PnL   float64 `json:"pnl"`
}

type Report struct {
	Initial     float64 `json:"initial"`
	FinalEquity float64 `json:"finalEquity"`
	ReturnPct   float64 `json:"returnPct"`
	MaxDrawdown float64 `json:"maxDrawdown"`
	Trades      int     `json:"trades"`
	WinRate     float64 `json:"winRate"`
	Curve       []Point `json:"curve"`
	Fills       []Fill  `json:"fills"`
}

func (p Params) Normalize(kind string) Params {
	if p.PositionPct <= 0 {
		p.PositionPct = 0.3
	}
	switch kind {
	case "ma_cross":
		if p.Fast <= 0 {
			p.Fast = 5
		}
		if p.Slow <= 0 {
			p.Slow = 20
		}
	case "rsi":
		if p.RSIPeriod <= 0 {
			p.RSIPeriod = 14
		}
		if p.Oversold <= 0 {
			p.Oversold = 30
		}
		if p.Overbought <= 0 {
			p.Overbought = 70
		}
	case "momentum":
		if p.Lookback <= 0 {
			p.Lookback = 10
		}
		if p.Threshold <= 0 {
			p.Threshold = 0.03
		}
		if p.MAPeriod <= 0 {
			p.MAPeriod = 20
		}
	}
	return p
}

func (p Params) Validate(kind string) error {
	switch kind {
	case "ma_cross", "rsi", "momentum":
	default:
		return fmt.Errorf("不支持的策略类型")
	}
	if p.PositionPct < 0.05 || p.PositionPct > 1 {
		return fmt.Errorf("仓位比例应在 5%% 到 100%% 之间")
	}
	switch kind {
	case "ma_cross":
		if p.Fast < 2 || p.Slow <= p.Fast || p.Slow > 120 {
			return fmt.Errorf("双均线参数需要 2 ≤ 短周期 < 长周期 ≤ 120")
		}
	case "rsi":
		if p.RSIPeriod < 2 || p.RSIPeriod > 40 || p.Oversold >= p.Overbought || p.Oversold <= 0 || p.Overbought >= 100 {
			return fmt.Errorf("RSI 参数不正确")
		}
	case "momentum":
		if p.Lookback < 2 || p.Lookback > 60 || p.MAPeriod < 2 || p.MAPeriod > 120 {
			return fmt.Errorf("动量参数不正确")
		}
	}
	return nil
}

func Latest(kind string, bars []Bar, p Params) (int, error) {
	series, err := SignalSeries(kind, bars, p)
	if err != nil {
		return 0, err
	}
	return series[len(series)-1], nil
}

func SignalSeries(kind string, bars []Bar, p Params) ([]int, error) {
	p = p.Normalize(kind)
	if err := p.Validate(kind); err != nil {
		return nil, err
	}
	if len(bars) < warmup(kind, p) {
		return nil, fmt.Errorf("K线不足以运行该策略")
	}
	closes := closesOf(bars)
	out := make([]int, len(bars))
	held := 0
	for i := range bars {
		switch kind {
		case "ma_cross":
			if i+1 < p.Slow {
				continue
			}
			fast, _ := sma(closes[:i+1], p.Fast)
			slow, _ := sma(closes[:i+1], p.Slow)
			if fast > slow {
				held = 1
			} else {
				held = 0
			}
		case "rsi":
			if i+1 <= p.RSIPeriod {
				continue
			}
			rsi := rsiWilder(closes[:i+1], p.RSIPeriod)
			switch {
			case rsi <= p.Oversold:
				held = 1
			case rsi >= p.Overbought:
				held = 0
			}
		case "momentum":
			need := p.Lookback
			if p.MAPeriod > need {
				need = p.MAPeriod
			}
			if i < need {
				continue
			}
			prev := closes[i-p.Lookback]
			ret := 0.0
			if prev > 0 {
				ret = closes[i]/prev - 1
			}
			ma, _ := sma(closes[:i+1], p.MAPeriod)
			if ret >= p.Threshold && closes[i] > ma {
				held = 1
			} else if ret < 0 || closes[i] < ma {
				held = 0
			}
		}
		out[i] = held
	}
	return out, nil
}

func Backtest(kind string, bars []Bar, initial float64, p Params) (Report, error) {
	if initial <= 0 {
		initial = 1_000_000
	}
	series, err := SignalSeries(kind, bars, p)
	if err != nil {
		return Report{}, err
	}
	p = p.Normalize(kind)
	cash := initial
	qty := 0
	costBasis := 0.0
	peak := initial
	maxDD := 0.0
	wins, sells := 0, 0
	fills := make([]Fill, 0)
	curve := make([]Point, 0, len(bars))
	for i, bar := range bars {
		price := bar.Close
		if price <= 0 {
			continue
		}
		if series[i] == 1 && qty == 0 {
			target := cash * p.PositionPct
			q := LotSize(target, price, cash)
			if q > 0 {
				amount := round2(float64(q) * price)
				fee := Commission("buy", amount)
				cash = round2(cash - amount - fee)
				qty = q
				costBasis = round2(amount + fee)
				fills = append(fills, Fill{Date: bar.Date, Side: "buy", Price: price, Qty: q, Fee: fee})
			}
		} else if series[i] == 0 && qty > 0 {
			amount := round2(float64(qty) * price)
			fee := Commission("sell", amount)
			proceeds := round2(amount - fee)
			pnl := round2(proceeds - costBasis)
			cash = round2(cash + proceeds)
			fills = append(fills, Fill{Date: bar.Date, Side: "sell", Price: price, Qty: qty, Fee: fee, PnL: pnl})
			sells++
			if pnl > 0 {
				wins++
			}
			qty = 0
			costBasis = 0
		}
		equity := round2(cash + float64(qty)*price)
		if equity > peak {
			peak = equity
		}
		if peak > 0 {
			dd := (peak - equity) / peak
			if dd > maxDD {
				maxDD = dd
			}
		}
		curve = append(curve, Point{Date: bar.Date, Equity: equity})
	}
	final := initial
	if len(curve) > 0 {
		final = curve[len(curve)-1].Equity
	}
	winRate := 0.0
	if sells > 0 {
		winRate = float64(wins) / float64(sells) * 100
	}
	return Report{
		Initial:     round2(initial),
		FinalEquity: round2(final),
		ReturnPct:   round2((final - initial) / initial * 100),
		MaxDrawdown: round2(maxDD * 100),
		Trades:      len(fills),
		WinRate:     round2(winRate),
		Curve:       downsample(curve, 180),
		Fills:       fills,
	}, nil
}

func Commission(side string, amount float64) float64 {
	fee := amount * 0.00025
	if fee < 5 {
		fee = 5
	}
	if side == "sell" {
		fee += amount * 0.0005
	}
	return round2(fee)
}

func LotSize(target, price, cash float64) int {
	if price <= 0 {
		return 0
	}
	qty := int(target/price/100) * 100
	for qty > 0 {
		amount := float64(qty) * price
		if amount+Commission("buy", amount) <= cash+1e-6 {
			return qty
		}
		qty -= 100
	}
	return 0
}

func warmup(kind string, p Params) int {
	switch kind {
	case "ma_cross":
		return p.Slow + 2
	case "rsi":
		return p.RSIPeriod + 2
	default:
		need := p.Lookback
		if p.MAPeriod > need {
			need = p.MAPeriod
		}
		return need + 2
	}
}

func closesOf(bars []Bar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.Close
	}
	return out
}

func sma(values []float64, period int) (float64, bool) {
	if period <= 0 || len(values) < period {
		return 0, false
	}
	var sum float64
	for _, v := range values[len(values)-period:] {
		sum += v
	}
	return sum / float64(period), true
}

func rsiWilder(closes []float64, period int) float64 {
	if len(closes) <= period {
		return 50
	}
	var gain, loss float64
	for i := 1; i <= period; i++ {
		d := closes[i] - closes[i-1]
		if d >= 0 {
			gain += d
		} else {
			loss -= d
		}
	}
	avgG := gain / float64(period)
	avgL := loss / float64(period)
	for i := period + 1; i < len(closes); i++ {
		d := closes[i] - closes[i-1]
		g, l := 0.0, 0.0
		if d >= 0 {
			g = d
		} else {
			l = -d
		}
		avgG = (avgG*float64(period-1) + g) / float64(period)
		avgL = (avgL*float64(period-1) + l) / float64(period)
	}
	if avgG == 0 && avgL == 0 {
		return 50
	}
	if avgL == 0 {
		return 100
	}
	rs := avgG / avgL
	return 100 - 100/(1+rs)
}

func downsample(points []Point, max int) []Point {
	if len(points) <= max || max < 2 {
		return points
	}
	step := float64(len(points)-1) / float64(max-1)
	out := make([]Point, 0, max)
	last := -1
	for i := 0; i < max; i++ {
		idx := int(math.Round(float64(i) * step))
		if idx == last {
			continue
		}
		last = idx
		out = append(out, points[idx])
	}
	return out
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
