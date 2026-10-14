package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"

	"nstock/internal/api"
	"nstock/internal/config"
	"nstock/internal/database"
	"nstock/internal/market"
	"nstock/internal/monitor"
	"nstock/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

func New() *fx.App {
	return fx.New(
		fx.Provide(
			config.Load,
			database.Open,
			func() market.Provider { return market.NewEastMoney() },
			service.New,
			monitor.New,
			api.NewEngine,
		),
		fx.Invoke(prepare, serve),
	)
}

func prepare(db *gorm.DB, svc *service.Service, lc fx.Lifecycle) error {
	if err := database.Migrate(db); err != nil {
		return err
	}
	if err := service.Seed(db); err != nil {
		return err
	}
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go svc.Warm(context.Background())
			return nil
		},
	})
	return nil
}

func serve(lc fx.Lifecycle, cfg config.Config, engine *gin.Engine, runner *monitor.Runner) {
	server := &http.Server{Addr: cfg.Addr, Handler: engine}
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			ln, err := net.Listen("tcp", cfg.Addr)
			if err != nil {
				return err
			}
			runner.Start()
			slog.Info("衡观已启动", "addr", cfg.Addr)
			go func() {
				if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
					slog.Error("HTTP 服务退出", "err", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			runner.Stop()
			return server.Shutdown(ctx)
		},
	})
}
