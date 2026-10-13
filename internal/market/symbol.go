package market

import (
	"fmt"
	"strings"
	"unicode"
)

func NormalizeCode(raw string) (string, error) {
	code := strings.TrimSpace(strings.ToLower(raw))
	for _, prefix := range []string{"sh", "sz", "bj"} {
		code = strings.TrimPrefix(code, prefix)
	}
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return "", fmt.Errorf("证券代码应为 6 位数字")
	}
	for _, r := range code {
		if !unicode.IsDigit(r) {
			return "", fmt.Errorf("证券代码应为 6 位数字")
		}
	}
	return code, nil
}

func InferMarket(code string) string {
	switch {
	case strings.HasPrefix(code, "8"), strings.HasPrefix(code, "4"), strings.HasPrefix(code, "92"):
		return "bj"
	case strings.HasPrefix(code, "6"), strings.HasPrefix(code, "9"):
		return "sh"
	default:
		return "sz"
	}
}

func SecID(code, market string) string {
	if market == "sh" {
		return "1." + code
	}
	return "0." + code
}

func MarketLabel(market string) string {
	switch market {
	case "sh":
		return "沪"
	case "bj":
		return "京"
	default:
		return "深"
	}
}
