package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"nstock/internal/config"
	"nstock/internal/monitor"
	"nstock/internal/service"

	"github.com/gin-gonic/gin"
)

type Server struct {
	svc    *service.Service
	runner *monitor.Runner
	cfg    config.Config
}

func NewEngine(svc *service.Service, runner *monitor.Runner, cfg config.Config) *gin.Engine {
	if os.Getenv("NSTOCK_GIN") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	engine.Use(gin.Recovery(), cors())
	server := &Server{svc: svc, runner: runner, cfg: cfg}
	api := engine.Group("/api")
	api.GET("/health", server.health)
	api.GET("/market/search", server.search)
	api.GET("/market/indices", server.indices)
	api.GET("/market/quotes", server.quotes)
	api.GET("/market/quote/:code", server.quote)
	api.GET("/market/kline/:code", server.kline)
	api.GET("/watchlist", server.watchlist)
	api.POST("/watchlist", server.addWatch)
	api.DELETE("/watchlist/:code", server.removeWatch)
	api.GET("/predict", server.predictions)
	api.POST("/predict/:code", server.predict)
	api.GET("/account", server.account)
	api.POST("/account/reset", server.resetAccount)
	api.GET("/orders", server.orders)
	api.POST("/orders", server.placeOrder)
	api.GET("/strategies", server.strategies)
	api.POST("/strategies", server.createStrategy)
	api.POST("/strategies/run-enabled", server.runEnabled)
	api.PUT("/strategies/:id", server.updateStrategy)
	api.DELETE("/strategies/:id", server.deleteStrategy)
	api.POST("/strategies/:id/backtest", server.backtest)
	api.POST("/strategies/:id/run", server.runStrategy)
	api.GET("/backtests", server.backtests)
	api.GET("/alerts", server.alerts)
	api.POST("/alerts", server.createAlert)
	api.PUT("/alerts/:id", server.updateAlert)
	api.DELETE("/alerts/:id", server.deleteAlert)
	api.GET("/alert-events", server.events)
	api.POST("/monitor/scan", server.scan)
	api.GET("/monitor/status", server.monitorStatus)
	server.pages(engine)
	return engine
}

func (s *Server) pages(engine *gin.Engine) {
	index := filepath.Join(s.cfg.WebDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		engine.GET("/", func(c *gin.Context) {
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("<!doctype html><meta charset=utf-8><p>前端还没有构建。请在 web 目录执行 npm install 和 npm run build。</p>"))
		})
		return
	}
	engine.Static("/assets", filepath.Join(s.cfg.WebDir, "assets"))
	engine.GET("/", func(c *gin.Context) { c.File(index) })
	engine.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
			return
		}
		c.File(index)
	})
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (s *Server) fail(c *gin.Context, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, service.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, service.ErrUpstream):
		status = http.StatusBadGateway
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

func paramID(c *gin.Context) (uint, bool) {
	n, err := strconv.Atoi(c.Param("id"))
	if err != nil || n <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "编号不正确"})
		return 0, false
	}
	return uint(n), true
}

func queryLimit(c *gin.Context, fallback int) int {
	n, err := strconv.Atoi(c.Query("limit"))
	if err != nil || n <= 0 {
		return fallback
	}
	if n > 200 {
		return 200
	}
	return n
}

func wantsRefresh(c *gin.Context) bool {
	switch c.Query("refresh") {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}
