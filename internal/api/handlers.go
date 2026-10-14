package api

import (
	"math"
	"net/http"
	"strconv"

	"nstock/internal/quant"
	"nstock/internal/service"

	"github.com/gin-gonic/gin"
)

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "service": "nstock", "dataSource": "eastmoney"})
}

func (s *Server) search(c *gin.Context) {
	hits, err := s.svc.Search(c.Request.Context(), c.Query("q"))
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": hits})
}

func (s *Server) indices(c *gin.Context) {
	rows, err := s.svc.Indices(c.Request.Context())
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (s *Server) quotes(c *gin.Context) {
	list, err := s.svc.WatchQuotes(c.Request.Context(), wantsRefresh(c))
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

func (s *Server) quote(c *gin.Context) {
	row, err := s.svc.Quote(c.Request.Context(), c.Param("code"), wantsRefresh(c) || c.Query("refresh") == "")
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, row)
}

func (s *Server) kline(c *gin.Context) {
	limit, _ := parseInt(c.Query("limit"))
	view, err := s.svc.Kline(c.Request.Context(), c.Param("code"), limit, c.Query("refresh") != "0")
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

func (s *Server) watchlist(c *gin.Context) {
	rows, err := s.svc.ListWatch()
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (s *Server) addWatch(c *gin.Context) {
	var body struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供证券代码或名称"})
		return
	}
	item, err := s.svc.AddWatch(c.Request.Context(), body.Code)
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (s *Server) removeWatch(c *gin.Context) {
	if err := s.svc.RemoveWatch(c.Param("code")); err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) predict(c *gin.Context) {
	view, err := s.svc.Predict(c.Request.Context(), c.Param("code"))
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

func (s *Server) predictions(c *gin.Context) {
	rows, err := s.svc.Predictions(c.Query("code"), queryLimit(c, 20))
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (s *Server) account(c *gin.Context) {
	view, err := s.svc.Account(c.Request.Context())
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

func (s *Server) resetAccount(c *gin.Context) {
	view, err := s.svc.ResetAccount(c.Request.Context())
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

func (s *Server) orders(c *gin.Context) {
	rows, err := s.svc.Orders(queryLimit(c, 50))
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (s *Server) placeOrder(c *gin.Context) {
	var body struct {
		Code string  `json:"code"`
		Side string  `json:"side"`
		Qty  float64 `json:"qty"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数不正确"})
		return
	}
	order, err := s.svc.PlaceOrder(c.Request.Context(), service.OrderInput{
		Code: body.Code, Side: body.Side, Qty: int(math.Round(body.Qty)), Source: "manual",
	})
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, order)
}

func (s *Server) strategies(c *gin.Context) {
	rows, err := s.svc.Strategies()
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (s *Server) createStrategy(c *gin.Context) {
	var body struct {
		Name    string       `json:"name"`
		Type    string       `json:"type"`
		Code    string       `json:"code"`
		Enabled *bool        `json:"enabled"`
		Params  quant.Params `json:"params"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Code == "" || body.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写策略类型和证券"})
		return
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	view, err := s.svc.CreateStrategy(c.Request.Context(), service.StrategyInput{
		Name: body.Name, Type: body.Type, Code: body.Code, Params: body.Params, Enabled: enabled,
	})
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

func (s *Server) updateStrategy(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var body struct {
		Name    *string       `json:"name"`
		Code    *string       `json:"code"`
		Enabled *bool         `json:"enabled"`
		Params  *quant.Params `json:"params"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数不正确"})
		return
	}
	view, err := s.svc.UpdateStrategy(c.Request.Context(), id, service.StrategyPatch{
		Name: body.Name, Code: body.Code, Enabled: body.Enabled, Params: body.Params,
	})
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

func (s *Server) deleteStrategy(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	if err := s.svc.DeleteStrategy(id); err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) backtest(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	view, err := s.svc.Backtest(c.Request.Context(), id)
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

func (s *Server) runStrategy(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	view, err := s.svc.RunStrategy(c.Request.Context(), id)
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

func (s *Server) runEnabled(c *gin.Context) {
	rows, err := s.svc.RunEnabled(c.Request.Context())
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (s *Server) backtests(c *gin.Context) {
	strategyID, _ := parseInt(c.Query("strategyId"))
	rows, err := s.svc.Backtests(uint(strategyID), queryLimit(c, 8))
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (s *Server) alerts(c *gin.Context) {
	rows, err := s.svc.Rules()
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (s *Server) createAlert(c *gin.Context) {
	var body struct {
		Code      string  `json:"code"`
		Kind      string  `json:"kind"`
		Threshold float64 `json:"threshold"`
		Name      string  `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数不正确"})
		return
	}
	row, err := s.svc.CreateRule(c.Request.Context(), service.RuleInput{
		Code: body.Code, Kind: body.Kind, Threshold: body.Threshold, Name: body.Name,
	})
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, row)
}

func (s *Server) updateAlert(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var body struct {
		Name      *string  `json:"name"`
		Enabled   *bool    `json:"enabled"`
		Threshold *float64 `json:"threshold"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数不正确"})
		return
	}
	row, err := s.svc.UpdateRule(id, service.RulePatch{Name: body.Name, Enabled: body.Enabled, Threshold: body.Threshold})
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, row)
}

func (s *Server) deleteAlert(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	if err := s.svc.DeleteRule(id); err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) events(c *gin.Context) {
	rows, err := s.svc.Events(queryLimit(c, 50))
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (s *Server) scan(c *gin.Context) {
	result, err := s.runner.Scan(c.Request.Context())
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) monitorStatus(c *gin.Context) {
	c.JSON(http.StatusOK, s.runner.Status())
}

func parseInt(raw string) (int, error) {
	return strconv.Atoi(raw)
}
