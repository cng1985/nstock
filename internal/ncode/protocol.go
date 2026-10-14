package ncode

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"
	"time"
)

const (
	CodeOK           = 200
	CodeBadRequest   = 400
	CodeUnauthorized = 401
	CodeForbidden    = 403
	CodeNotFound     = 404
	CodeConflict     = 409
	CodeLimited      = 429
	CodeError        = 500

	CodeInsufficient = 20001
)

type Error struct {
	Code int
	Msg  string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Msg
}

func Fail(code int, msg string) *Error {
	return &Error{Code: code, Msg: msg}
}

type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

type PageResponse struct {
	Code         int    `json:"code"`
	Msg          string `json:"msg"`
	Data         any    `json:"data"`
	Page         int    `json:"page"`
	Size         int    `json:"size"`
	Total        int64  `json:"total"`
	TotalPage    int    `json:"totalPage"`
	ResponseType string `json:"responseType"`
}

type QueryRequest struct {
	Model   string         `json:"model"`
	Page    int            `json:"page"`
	Size    int            `json:"size"`
	Filters map[string]any `json:"filters"`
	Sort    *Sort          `json:"sort"`
}

type Sort struct {
	Field  string `json:"field"`
	Method string `json:"method"`
}

var shanghai = time.FixedZone("CST", 8*3600)

func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(shanghai).Format("2006-01-02 15:04:05")
}

func NormalizePage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 200 {
		size = 200
	}
	return page, size
}

func TotalPage(total int64, size int) int {
	if size <= 0 || total <= 0 {
		return 0
	}
	return int(math.Ceil(float64(total) / float64(size)))
}

func CheckTenant(raw []byte) error {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}
	var probe struct {
		TenantID json.RawMessage            `json:"tenantId"`
		Filters  map[string]json.RawMessage `json:"filters"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return Fail(CodeBadRequest, "请求参数不正确")
	}
	if tenantSet(probe.TenantID) {
		return Fail(CodeBadRequest, "不能由客户端指定租户")
	}
	if probe.Filters != nil && tenantSet(probe.Filters["tenantId"]) {
		return Fail(CodeBadRequest, "不能由客户端指定租户")
	}
	return nil
}

func tenantSet(raw json.RawMessage) bool {
	text := strings.TrimSpace(string(raw))
	return text != "" && text != "null"
}

func Authorize(header, configuredToken, permission string) error {
	header = strings.TrimSpace(header)
	if header != "" && !strings.HasPrefix(header, "Bearer ") {
		return Fail(CodeUnauthorized, "认证方式应为 Bearer token")
	}
	if configuredToken == "" {
		return nil
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if !strings.HasPrefix(header, "Bearer ") || token != configuredToken {
		return Fail(CodeUnauthorized, "未认证")
	}
	if permission == "" {
		return Fail(CodeForbidden, "无权限")
	}
	return nil
}

func SanitizeFilters(filters map[string]any, allow []string) (map[string]any, error) {
	out := map[string]any{}
	if len(filters) == 0 {
		return out, nil
	}
	ok := map[string]struct{}{}
	for _, name := range allow {
		ok[name] = struct{}{}
	}
	for key, value := range filters {
		if key == "tenantId" {
			return nil, Fail(CodeBadRequest, "不能由客户端指定租户")
		}
		if _, allowed := ok[key]; !allowed {
			return nil, Fail(CodeBadRequest, "不支持的查询条件: "+key)
		}
		out[key] = value
	}
	return out, nil
}

func CheckSort(sort *Sort, allow []string) error {
	if sort == nil || (sort.Field == "" && sort.Method == "") {
		return nil
	}
	if sort.Method != "asc" && sort.Method != "desc" {
		return Fail(CodeBadRequest, "排序方式只能是 asc 或 desc")
	}
	for _, field := range allow {
		if field == sort.Field {
			return nil
		}
	}
	return Fail(CodeBadRequest, "不支持的排序字段")
}

func Ascending(sort *Sort) bool {
	return sort != nil && sort.Method == "asc"
}

func Write(w http.ResponseWriter, err error, data any) {
	if err != nil {
		code, msg := MapError(err)
		write(w, statusOf(code), Response{Code: code, Msg: msg, Data: nil})
		return
	}
	write(w, http.StatusOK, Response{Code: CodeOK, Msg: "success", Data: data})
}

func WritePage(w http.ResponseWriter, err error, data any, page, size int, total int64) {
	if err != nil {
		Write(w, err, nil)
		return
	}
	if data == nil {
		data = []any{}
	}
	page, size = NormalizePage(page, size)
	write(w, http.StatusOK, PageResponse{
		Code: CodeOK, Msg: "success", Data: data,
		Page: page, Size: size, Total: total, TotalPage: TotalPage(total, size),
		ResponseType: "page",
	})
}

func MapError(err error) (int, string) {
	var biz *Error
	if errors.As(err, &biz) {
		return biz.Code, biz.Msg
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "现金不足"), strings.Contains(msg, "持仓不足"):
		return CodeInsufficient, msg
	case strings.Contains(msg, "未找到"), strings.Contains(msg, "不存在"):
		return CodeNotFound, msg
	case strings.Contains(msg, "行情源"):
		return CodeError, msg
	default:
		return CodeBadRequest, msg
	}
}

func statusOf(code int) int {
	if code >= 400 && code <= 599 {
		return code
	}
	if code == CodeOK {
		return http.StatusOK
	}
	return http.StatusBadRequest
}

func write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func AsString(v any) string {
	switch value := v.(type) {
	case string:
		return strings.TrimSpace(value)
	default:
		return ""
	}
}

func AsBool(v any) bool {
	switch value := v.(type) {
	case bool:
		return value
	case string:
		return value == "1" || value == "true"
	case float64:
		return value != 0
	default:
		return false
	}
}

func AsInt(v any) int {
	switch value := v.(type) {
	case float64:
		return int(value)
	case int:
		return value
	case string:
		n := 0
		sign := 1
		for i, r := range value {
			if i == 0 && r == '-' {
				sign = -1
				continue
			}
			if r < '0' || r > '9' {
				return 0
			}
			n = n*10 + int(r-'0')
		}
		return n * sign
	default:
		return 0
	}
}

func AsUint(v any) uint {
	n := AsInt(v)
	if n < 0 {
		return 0
	}
	return uint(n)
}
