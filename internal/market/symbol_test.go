package market

import "testing"

func TestNormalizeCode(t *testing.T) {
	code, err := NormalizeCode("SH600519")
	if err != nil || code != "600519" {
		t.Fatalf("got %s %v", code, err)
	}
	if _, err := NormalizeCode("茅台"); err == nil {
		t.Fatal("expected error")
	}
}

func TestInferMarket(t *testing.T) {
	if InferMarket("600519") != "sh" || SecID("600519", "sh") != "1.600519" {
		t.Fatal("shanghai")
	}
	if InferMarket("000001") != "sz" || SecID("000001", "sz") != "0.000001" {
		t.Fatal("shenzhen")
	}
	if InferMarket("830000") != "bj" {
		t.Fatal("beijing")
	}
}
