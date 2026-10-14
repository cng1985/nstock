package config

import "go.uber.org/fx"

// Module 提供进程配置。
var Module = fx.Module("config",
	fx.Provide(Load),
)
