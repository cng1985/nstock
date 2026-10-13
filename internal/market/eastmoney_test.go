package market

import "testing"

func TestParseQuotesAndKline(t *testing.T) {
	body := []byte(`{"data":{"diff":[{"f2":10.5,"f3":1.2,"f4":0.12,"f5":100,"f6":1000,"f12":"000001","f13":0,"f14":"平安银行","f15":10.8,"f16":10.2,"f17":10.4,"f18":10.38}]}}`)
	quotes, err := parseQuotes(body)
	if err != nil || len(quotes) != 1 || quotes[0].SecID != "0.000001" || quotes[0].Price != 10.5 {
		t.Fatalf("%v %+v", err, quotes)
	}
	kbody := []byte(`{"data":{"code":"600519","name":"贵州茅台","klines":["2026-10-13,1287.00,1274.89,1288.48,1268.11,26573,3393315572.00,1.58,-1.02,-13.11,0.21"]}}`)
	k, err := parseKline(kbody)
	if err != nil || k.Name != "贵州茅台" || k.Bars[0].Close != 1274.89 || k.Bars[0].High != 1288.48 {
		t.Fatalf("%v %+v", err, k)
	}
}

func TestParseSearch(t *testing.T) {
	body := []byte(`{"QuotationCodeTable":{"Data":[{"Code":"600519","Name":"贵州茅台","QuoteID":"1.600519","Classify":"AStock","SecurityTypeName":"沪A","MktNum":"1"},{"Code":"159919","Name":"沪深300ETF","Classify":"Fund","SecurityTypeName":"基金"}]}}`)
	hits, err := parseSearch(body)
	if err != nil || len(hits) != 1 || hits[0].SecID != "1.600519" {
		t.Fatalf("%v %+v", err, hits)
	}
}
