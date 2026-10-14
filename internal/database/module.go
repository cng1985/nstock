package database

import "go.uber.org/fx"

// Module 打开 SQLite，并在启动前完成表结构迁移。
var Module = fx.Module("database",
	fx.Provide(Open),
	fx.Invoke(Migrate),
)
