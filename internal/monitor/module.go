package monitor

import (
	"context"

	"go.uber.org/fx"
)

// Module 提供盯盘循环，并在进程启停时开关扫描。
var Module = fx.Module("monitor",
	fx.Provide(New),
	fx.Invoke(run),
)

func run(lc fx.Lifecycle, runner *Runner) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			runner.Start()
			return nil
		},
		OnStop: func(context.Context) error {
			runner.Stop()
			return nil
		},
	})
}
