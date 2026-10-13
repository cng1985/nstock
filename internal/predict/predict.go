package predict

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

type Reason struct {
	Name   string  `json:"name"`
	Score  float64 `json:"score"`
	Detail string  `json:"detail"`
}

type Result struct {
	Direction  string   `json:"direction"`
	Confidence float64  `json:"confidence"`
	Score      float64  `json:"score"`
	Close      float64  `json:"close"`
	Reasons    []Reason `json:"reasons"`
}

const minBars = 35

func Run(bars []Bar) (Result, error) {
	if len(bars) < minBars {
		return Result{}, fmt.Errorf("历史K线不足，至少需要 %d 根日线", minBars)
	}
	closes := make([]float64, len(bars))
	for i, b := range bars {
		closes[i] = b.Close
	}
	last := bars[len(bars)-1]
	prev := bars[len(bars)-2]

	ma5, _ := sma(closes, 5)
	ma20, _ := sma(closes, 20)
	maScore := math.Tanh((ma5 - ma20) / ma20 * 25)
	reasons := []Reason{{
		Name:   "均线趋势",
		Score:  round(maScore),
		Detail: fmt.Sprintf("MA5 %.2f，MA20 %.2f", ma5, ma20),
	}}

	hist, prevHist := macdHist(closes)
	macdScore := math.Tanh(hist/last.Close*80)*0.7 + math.Tanh((hist-prevHist)/last.Close*120)*0.3
	reasons = append(reasons, Reason{
		Name:   "MACD 动能",
		Score:  round(macdScore),
		Detail: fmt.Sprintf("柱值 %.4f，前一日 %.4f", hist, prevHist),
	})

	rsi := rsiWilder(closes, 14)
	var rsiScore float64
	switch {
	case rsi >= 75:
		rsiScore = -0.35
	case rsi <= 25:
		rsiScore = 0.35
	default:
		rsiScore = (rsi - 50) / 40
	}
	reasons = append(reasons, Reason{
		Name:   "RSI 强弱",
		Score:  round(rsiScore),
		Detail: fmt.Sprintf("RSI(14) %.1f", rsi),
	})

	base := closes[len(closes)-6]
	mom := 0.0
	if base > 0 {
		mom = math.Tanh((last.Close/base - 1) * 12)
	}
	reasons = append(reasons, Reason{
		Name:   "短期动量",
		Score:  round(mom),
		Detail: fmt.Sprintf("近 5 日涨跌 %.2f%%", (last.Close/base-1)*100),
	})

	volScore := volumeScore(bars)
	reasons = append(reasons, Reason{
		Name:   "量能确认",
		Score:  round(volScore),
		Detail: volumeDetail(bars, prev.Close, last.Close),
	})

	score := maScore*0.28 + macdScore*0.22 + rsiScore*0.18 + mom*0.22 + volScore*0.10
	score = clamp(score, -1, 1)
	dir := "flat"
	switch {
	case score > 0.08:
		dir = "up"
	case score < -0.08:
		dir = "down"
	}
	conf := math.Abs(math.Tanh(score * 1.35))
	return Result{
		Direction:  dir,
		Confidence: round(conf),
		Score:      round(score),
		Close:      last.Close,
		Reasons:    reasons,
	}, nil
}

func volumeScore(bars []Bar) float64 {
	n := 20
	if len(bars) <= n {
		return 0
	}
	var sum float64
	for _, b := range bars[len(bars)-1-n : len(bars)-1] {
		sum += b.Volume
	}
	avg := sum / float64(n)
	if avg <= 0 {
		return 0
	}
	ratio := bars[len(bars)-1].Volume / avg
	last := bars[len(bars)-1].Close
	prev := bars[len(bars)-2].Close
	if prev == 0 || last == prev {
		return 0
	}
	sign := 1.0
	if last < prev {
		sign = -1
	}
	if ratio >= 1 {
		return clamp((ratio-1)/1.5, 0, 1) * sign
	}
	return -0.15
}

func volumeDetail(bars []Bar, prev, last float64) string {
	var sum float64
	for _, b := range bars[len(bars)-21 : len(bars)-1] {
		sum += b.Volume
	}
	avg := sum / 20
	ratio := 0.0
	if avg > 0 {
		ratio = bars[len(bars)-1].Volume / avg
	}
	chg := 0.0
	if prev > 0 {
		chg = (last/prev - 1) * 100
	}
	return fmt.Sprintf("今日量能是 20 日均量的 %.2f 倍，当日涨跌 %.2f%%", ratio, chg)
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

func emaSeries(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if len(values) == 0 || period <= 0 {
		return out
	}
	k := 2.0 / float64(period+1)
	var seed float64
	if len(values) < period {
		for i, v := range values {
			seed += v
			out[i] = seed / float64(i+1)
		}
		return out
	}
	for i := 0; i < period; i++ {
		seed += values[i]
		out[i] = seed / float64(i+1)
	}
	out[period-1] = seed / float64(period)
	for i := period; i < len(values); i++ {
		out[i] = values[i]*k + out[i-1]*(1-k)
	}
	return out
}

func macdHist(closes []float64) (float64, float64) {
	ema12 := emaSeries(closes, 12)
	ema26 := emaSeries(closes, 26)
	macd := make([]float64, len(closes))
	for i := range closes {
		macd[i] = ema12[i] - ema26[i]
	}
	sig := emaSeries(macd, 9)
	n := len(closes)
	return macd[n-1] - sig[n-1], macd[n-2] - sig[n-2]
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

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func round(v float64) float64 {
	return math.Round(v*10000) / 10000
}
