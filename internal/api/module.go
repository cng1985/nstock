package api

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"

	"nstock/internal/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

// Module 提供 Gin 路由，并负责 HTTP 服务的监听与关闭。
var Module = fx.Module("http",
	fx.Provide(NewEngine),
	fx.Invoke(serve),
)

func serve(lc fx.Lifecycle, cfg config.Config, engine *gin.Engine) {
	server := &http.Server{Addr: cfg.Addr, Handler: engine}
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			ln, err := net.Listen("tcp", cfg.Addr)
			if err != nil {
				return err
			}
			slog.Info("衡观已启动", "addr", cfg.Addr)
			go func() {
				if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
					slog.Error("HTTP 服务退出", "err", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Shutdown(ctx)
		},
	})
}
