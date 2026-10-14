package quant

import (
	"math"
	"testing"
)

func TestBacktestLongOnlyLots(t *testing.T) {
	bars := make([]Bar, 80)
	for i := range bars {
		price := 20 + 0.08*float64(i) + 1.8*math.Sin(float64(i)/4)
		bars[i] = Bar{Date: "2024-01-02", Close: price, Open: price, High: price, Low: price, Volume: 10000}
	}
	p := Params{Fast: 5, Slow: 20, PositionPct: 0.5}.Normalize("ma_cross")
	report, err := Backtest("ma_cross", bars, 1_000_000, p)
	if err != nil {
		t.Fatal(err)
	}
	if report.Trades == 0 || report.FinalEquity <= 0 {
		t.Fatalf("%+v", report)
	}
	qty := 0
	for _, fill := range report.Fills {
		if fill.Qty%100 != 0 || fill.Qty <= 0 {
			t.Fatalf("lot %d", fill.Qty)
		}
		if fill.Side == "buy" {
			qty += fill.Qty
		} else {
			qty -= fill.Qty
		}
		if qty < 0 {
			t.Fatal("short")
		}
	}
}

func TestRSIAndReject(t *testing.T) {
	if err := (Params{PositionPct: 0.01}).Normalize("ma_cross").Validate("ma_cross"); err == nil {
		t.Fatal("expected invalid position")
	}
	bars := make([]Bar, 50)
	for i := range bars {
		bars[i] = Bar{Close: 10 + float64(i%7), Date: "d"}
	}
	_, err := Latest("rsi", bars, Params{RSIPeriod: 14, Oversold: 30, Overbought: 70, PositionPct: 0.2})
	if err != nil {
		t.Fatal(err)
	}
}
