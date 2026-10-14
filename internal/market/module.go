package market

import "go.uber.org/fx"

// Module 提供在线行情源。
var Module = fx.Module("market",
	fx.Provide(fx.Annotate(NewEastMoney, fx.As(new(Provider)))),
)
