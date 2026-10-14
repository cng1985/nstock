package app

import (
	"nstock/internal/api"
	"nstock/internal/auth"
	"nstock/internal/config"
	"nstock/internal/database"
	"nstock/internal/market"
	"nstock/internal/monitor"
	"nstock/internal/service"

	"go.uber.org/fx"
)

// New 按功能模块组装应用。各模块自己声明依赖和生命周期。
func New() *fx.App {
	return fx.New(Options()...)
}

// Options 是可单独校验的模块图。
func Options() []fx.Option {
	return []fx.Option{
		config.Module,
		database.Module,
		auth.Module,
		market.Module,
		service.Module,
		monitor.Module,
		api.Module,
	}
}
