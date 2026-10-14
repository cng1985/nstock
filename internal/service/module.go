package service

import (
	"context"

	"go.uber.org/fx"
)

// Module 提供行情、预测、模拟交易和盯盘规则，并在启动时灌入种子数据。
var Module = fx.Module("service",
	fx.Provide(New),
	fx.Invoke(Seed, warmOnStart),
)

func warmOnStart(lc fx.Lifecycle, svc *Service) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go svc.Warm(context.Background())
			return nil
		},
	})
}
