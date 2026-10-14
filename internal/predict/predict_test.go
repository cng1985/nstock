package predict

import "testing"

func barsFrom(closes []float64) []Bar {
	out := make([]Bar, len(closes))
	for i, c := range closes {
		out[i] = Bar{Date: "2024-01-01", Close: c, Open: c, High: c, Low: c, Volume: 1000}
	}
	return out
}

func TestRunTrend(t *testing.T) {
	up := make([]float64, 40)
	down := make([]float64, 40)
	flat := make([]float64, 40)
	for i := 0; i < 40; i++ {
		up[i] = 100 + float64(i)
		down[i] = 140 - float64(i)
		flat[i] = 100
	}
	got, err := Run(barsFrom(up))
	if err != nil || got.Direction != "up" || got.Confidence < 0.3 {
		t.Fatalf("up: %+v %v", got, err)
	}
	got, err = Run(barsFrom(down))
	if err != nil || got.Direction != "down" {
		t.Fatalf("down: %+v %v", got, err)
	}
	got, err = Run(barsFrom(flat))
	if err != nil || got.Direction != "flat" {
		t.Fatalf("flat: %+v %v", got, err)
	}
}

func TestRunNeedsHistory(t *testing.T) {
	if _, err := Run(barsFrom([]float64{1, 2, 3})); err == nil {
		t.Fatal("expected error")
	}
}
