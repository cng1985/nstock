package auth

import (
	"nstock/internal/database"

	"go.uber.org/fx"
	"gorm.io/gorm"
)

// Module 提供登录会话和 RBAC，并在表结构就绪后写入内置角色与初始账号。
var Module = fx.Module("auth",
	fx.Provide(New),
	fx.Invoke(seedOnStart),
)

func seedOnStart(db *gorm.DB, _ database.Schema) error {
	return Seed(db)
}
