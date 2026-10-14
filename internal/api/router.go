package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"nstock/internal/config"
	"nstock/internal/monitor"
	"nstock/internal/ncode"
	"nstock/internal/service"
	"nstock/web"

	"github.com/gin-gonic/gin"
)

type Server struct {
	svc    *service.Service
	runner *monitor.Runner
	cfg    config.Config
	models map[string]queryModel
}

func NewEngine(svc *service.Service, runner *monitor.Runner, cfg config.Config) *gin.Engine {
	if os.Getenv("NSTOCK_GIN") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	engine.Use(gin.Recovery(), cors())
	server := &Server{svc: svc, runner: runner, cfg: cfg}
	server.models = server.queryModels()
	v1 := engine.Group("/api/v1")
	v1.POST("/system/health", server.health)
	v1.POST("/market/search", server.search)
	v1.POST("/market/indices", server.indices)
	v1.POST("/quote/list", server.quoteList)
	v1.POST("/quote/get", server.quoteGet)
	v1.POST("/kline/get", server.klineGet)
	v1.POST("/watch/list", server.watchList)
	v1.POST("/watch/create", server.watchCreate)
	v1.POST("/watch/delete", server.watchDelete)
	v1.POST("/predict/execute", server.predictExecute)
	v1.POST("/predict/search", server.predictSearch)
	v1.POST("/account/get", server.accountGet)
	v1.POST("/account/reset", server.accountReset)
	v1.POST("/order/create", server.orderCreate)
	v1.POST("/order/search", server.orderSearch)
	v1.POST("/strategy/list", server.strategyList)
	v1.POST("/strategy/create", server.strategyCreate)
	v1.POST("/strategy/update", server.strategyUpdate)
	v1.POST("/strategy/delete", server.strategyDelete)
	v1.POST("/strategy/backtest", server.strategyBacktest)
	v1.POST("/strategy/execute", server.strategyExecute)
	v1.POST("/strategy/executeEnabled", server.strategyExecuteEnabled)
	v1.POST("/backtest/search", server.backtestSearch)
	v1.POST("/alert/list", server.alertList)
	v1.POST("/alert/create", server.alertCreate)
	v1.POST("/alert/update", server.alertUpdate)
	v1.POST("/alert/delete", server.alertDelete)
	v1.POST("/alertEvent/search", server.alertEventSearch)
	v1.POST("/monitor/scan", server.monitorScan)
	v1.POST("/monitor/get", server.monitorGet)
	v1.POST("/query/search", server.querySearch)
	v1.POST("/query/list", server.queryList)
	v1.POST("/query/one", server.queryOne)
	server.pages(engine)
	return engine
}

func (s *Server) pages(engine *gin.Engine) {
	index := filepath.Join(s.cfg.WebDir, "index.html")
	if st, err := os.Stat(index); err == nil && !st.IsDir() {
		s.pagesDir(engine, index)
		return
	}
	s.pagesEmbed(engine)
}

func (s *Server) pagesDir(engine *gin.Engine, index string) {
	engine.Static("/assets", filepath.Join(s.cfg.WebDir, "assets"))
	engine.GET("/", func(c *gin.Context) { c.File(index) })
	engine.NoRoute(func(c *gin.Context) {
		if apiMissing(c) {
			return
		}
		c.File(index)
	})
}

func (s *Server) pagesEmbed(engine *gin.Engine) {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		s.pagesMissing(engine)
		return
	}
	index, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		s.pagesMissing(engine)
		return
	}
	files := http.FileServer(http.FS(dist))
	engine.NoRoute(func(c *gin.Context) {
		if apiMissing(c) {
			return
		}
		name := strings.TrimPrefix(path.Clean(c.Request.URL.Path), "/")
		if name != "" && name != "." {
			if info, err := fs.Stat(dist, name); err == nil && !info.IsDir() {
				files.ServeHTTP(c.Writer, c.Request)
				return
			}
			if strings.HasPrefix(name, "assets/") || path.Ext(name) != "" {
				c.Status(http.StatusNotFound)
				return
			}
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	})
}

func (s *Server) pagesMissing(engine *gin.Engine) {
	engine.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("<!doctype html><meta charset=utf-8><p>前端还没有构建。请在 web 目录执行 npm install 和 npm run build。</p>"))
	})
}

func apiMissing(c *gin.Context) bool {
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		ncode.Write(c.Writer, ncode.Fail(ncode.CodeNotFound, "接口不存在"), nil)
		return true
	}
	return false
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (s *Server) handle(c *gin.Context, permission string, dest any, fn func(context.Context) (any, error)) {
	if _, err := s.prepare(c, permission, dest); err != nil {
		ncode.Write(c.Writer, err, nil)
		return
	}
	data, err := fn(c.Request.Context())
	ncode.Write(c.Writer, err, data)
}

func (s *Server) handlePage(c *gin.Context, permission string, dest any, fn func(context.Context) (any, int, int, int64, error)) {
	if _, err := s.prepare(c, permission, dest); err != nil {
		ncode.Write(c.Writer, err, nil)
		return
	}
	data, page, size, total, err := fn(c.Request.Context())
	ncode.WritePage(c.Writer, err, data, page, size, total)
}

func (s *Server) prepare(c *gin.Context, permission string, dest any) ([]byte, error) {
	raw, err := readBody(c)
	if err != nil {
		return nil, err
	}
	if err = ncode.CheckTenant(raw); err != nil {
		return nil, err
	}
	if err = ncode.Authorize(c.GetHeader("Authorization"), s.cfg.Token, permission); err != nil {
		return nil, err
	}
	if err = bind(raw, dest); err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *Server) gate(c *gin.Context, raw []byte, permission string) error {
	if err := ncode.CheckTenant(raw); err != nil {
		return err
	}
	return ncode.Authorize(c.GetHeader("Authorization"), s.cfg.Token, permission)
}

func readBody(c *gin.Context) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		return nil, ncode.Fail(ncode.CodeBadRequest, "请求参数不正确")
	}
	return raw, nil
}

func bind(raw []byte, dest any) error {
	if dest == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return ncode.Fail(ncode.CodeBadRequest, "请求参数不正确")
	}
	return nil
}
