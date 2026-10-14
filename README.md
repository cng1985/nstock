# 衡观 NSTOCK

基于 Go（Uber Fx、Gin、GORM、SQLite）和 Vue 3 的股票研究终端。行情来自东方财富公开接口，涨跌预测、量化交易和盯盘都在本地完成。

预测是技术信号，量化下单只进入模拟账户，不会连接券商，也不构成投资建议。

## 功能

- 行情：搜索 A 股、自选、指数和日 K
- 预测：均线、MACD、RSI、短期动量、量能，输出下一交易日方向和置信度
- 量化：双均线、RSI 回归、动量突破；历史回测和模拟成交
- 盯盘：价格和涨跌幅条件，后台定时扫描，触发后写入提醒

模拟成交按 100 股一手。买入收佣金（万 2.5，最低 5 元），卖出另计 0.05% 印花税。策略只做多。

## 运行

```bash
cd web && npm install && npm run build
go run ./cmd/server
```

打开 <http://localhost:8080>。

开发时可以另开前端：

```bash
go run ./cmd/server
cd web && npm run dev
```

前端开发服务器在 <http://localhost:5173>，接口代理到 8080。

```bash
go test ./...
```

## 配置

| 环境变量 | 默认 | 说明 |
| --- | --- | --- |
| `NSTOCK_ADDR` | `:8080` | 监听地址 |
| `NSTOCK_DB` | `data/nstock.db` | SQLite 文件 |
| `NSTOCK_WEB` | `web/dist` | 前端构建目录 |
| `NSTOCK_MONITOR_INTERVAL` | `20` | 盯盘间隔，秒，最小 5 |
| `NSTOCK_GIN` | 空 | 设为 `debug` 时打开 Gin 调试日志 |

SQLite 通过 GORM 使用纯 Go 驱动 [glebarez/sqlite](https://github.com/glebarez/sqlite)，不依赖 CGO。

## 结构

- `cmd/server` 启动 Fx 应用
- `internal/market` 东方财富行情
- `internal/predict` 预测打分
- `internal/quant` 回测和仓位计算
- `internal/service` 自选、模拟账户、策略、盯盘
- `web` Vue 3 界面
