package market

import "go.uber.org/fx"

// Module 提供在线行情，并在东方财富不可用时切换到启动数据源。
var Module = fx.Module("market",
	fx.Provide(NewEastMoney),
	fx.Provide(NewStartup),
	fx.Provide(fx.Annotate(NewFailover, fx.As(new(Provider)))),
)
