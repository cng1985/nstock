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

## 接口

业务接口统一为 `POST /api/v1/{资源}/{动作}`，请求体是 JSON，响应固定为：

```json
{"code":200,"msg":"success","data":{}}
```

分页接口额外返回 `page`、`size`、`total`、`totalPage` 和 `responseType: "page"`。列表、单条和分页也可以走统一查询：

```json
POST /api/v1/query/search
{"model":"pageForOrder","page":1,"size":20,"filters":{"code":"600519"}}
```

查询条件只接受模型白名单，不会拼进 SQL。请求里不能带 `tenantId`。设置 `NSTOCK_TOKEN` 后，调用需要 `Authorization: Bearer <token>`。

## 配置

| 环境变量 | 默认 | 说明 |
| --- | --- | --- |
| `NSTOCK_ADDR` | `:8080` | 监听地址 |
| `NSTOCK_DB` | `data/nstock.db` | SQLite 文件 |
| `NSTOCK_WEB` | `web/dist` | 前端构建目录 |
| `NSTOCK_MONITOR_INTERVAL` | `20` | 盯盘间隔，秒，最小 5 |
| `NSTOCK_GIN` | 空 | 设为 `debug` 时打开 Gin 调试日志 |
| `NSTOCK_TOKEN` | 空 | 非空时要求 Bearer 认证 |

SQLite 通过 GORM 使用纯 Go 驱动 [glebarez/sqlite](https://github.com/glebarez/sqlite)，不依赖 CGO。

## 结构

启动入口只组装 Fx 模块，每个包用 `var Module` 声明自己的依赖和生命周期：

- `internal/config` 配置
- `internal/database` SQLite 与迁移
- `internal/market` 东方财富行情
- `internal/service` 自选、预测、模拟账户和策略
- `internal/monitor` 盯盘循环
- `internal/api` HTTP 服务
- `internal/predict`、`internal/quant` 纯计算，不进入依赖注入
- `web` Vue 3 界面
