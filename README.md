# 衡观 NSTOCK

基于 Go（Uber Fx、Gin、GORM、SQLite）和 Vue 3 的股票研究终端。行情优先来自东方财富公开接口；这个源不可用时自动切换到随程序启动的内置样本行情，恢复后自动切回。涨跌预测、量化交易和盯盘都在本地完成。

预测是技术信号，量化下单只进入模拟账户，不会连接券商，也不构成投资建议。

## 功能

- 行情：搜索 A 股、自选、指数和日 K。东方财富中断时，自选仍可用启动数据源继续查询
- 预测：均线、MACD、RSI、短期动量、量能，输出下一交易日方向和置信度
- 量化：双均线、RSI 回归、动量突破；历史回测和模拟成交
- 盯盘：价格和涨跌幅条件，后台定时扫描，触发后写入提醒
- 登录和 RBAC：用户、角色、权限。菜单和按钮按权限显示，接口再校验一次

模拟成交按 100 股一手。买入收佣金（万 2.5，最低 5 元），卖出另计 0.05% 印花税。策略只做多。

## 运行

前端在编译 Go 时打进二进制（`web` 包里的 `//go:embed all:dist`）。仓库里已带一份构建结果，可以直接：

```bash
go run ./cmd/server
```

改了 Vue 之后要先重新构建，再编译 Go，新的页面才会进二进制：

```bash
cd web && npm install && npm run build
go run ./cmd/server
```

打开 <http://localhost:8080>。编译出的程序单独拷走也能打开界面，不必再带 `web/dist`。

若 `NSTOCK_WEB`（默认 `web/dist`）里已有 `index.html`，则优先用磁盘上的文件，这样只改前端时不必重新编译 Go。

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

查询条件只接受模型白名单，不会拼进 SQL。请求里不能带 `tenantId`。除 `POST /api/v1/auth/login` 外，请求都要带登录会话：`Authorization: Bearer <token>`。会话默认 7 天。若设置了 `NSTOCK_TOKEN`，这个静态令牌仍可访问全部接口，普通登录会话继续按角色限制。

登录返回 token、角色和权限码。当前用户用 `POST /api/v1/auth/me`，退出用 `POST /api/v1/auth/logout`，修改自己的密码用 `POST /api/v1/auth/password`。用户、角色和权限目录分别是 `/api/v1/user/*`、`/api/v1/role/*`、`/api/v1/permission/list`。

内置角色：

| 角色 | 范围 |
| --- | --- |
| 管理员 | 全部权限，包括用户和角色。不能取消全部权限，也不能停用或删除最后一名启用的管理员 |
| 交易员 | 行情、预测、模拟下单、策略和盯盘。不能重置账户，也不能管理用户 |
| 研究员 | 查看行情、账户、委托、策略和盯盘，可以生成预测。不能下单或改规则 |

只有管理员角色可以分配、修改或删除管理员。权限页可以新建自定义角色，并勾选权限码。

## 配置

| 环境变量 | 默认 | 说明 |
| --- | --- | --- |
| `NSTOCK_ADDR` | `:8080` | 监听地址 |
| `NSTOCK_DB` | `data/nstock.db` | SQLite 文件 |
| `NSTOCK_WEB` | `web/dist` | 若该目录有 `index.html` 则用它，否则用嵌进二进制的前端 |
| `NSTOCK_MONITOR_INTERVAL` | `20` | 盯盘间隔，秒，最小 5 |
| `NSTOCK_GIN` | 空 | 设为 `debug` 时打开 Gin 调试日志 |
| `NSTOCK_TOKEN` | 空 | 非空时，该静态 Bearer 令牌拥有全部权限 |

SQLite 通过 GORM 使用纯 Go 驱动 [glebarez/sqlite](https://github.com/glebarez/sqlite)，不依赖 CGO。

## 结构

启动入口只组装 Fx 模块，每个包用 `var Module` 声明自己的依赖和生命周期：

- `internal/config` 配置
- `internal/database` SQLite 与迁移
- `internal/market` 东方财富行情，失败时切换到启动数据源
- `internal/service` 自选、预测、模拟账户和策略
- `internal/monitor` 盯盘循环
- `internal/api` HTTP 服务
- `internal/predict`、`internal/quant` 纯计算，不进入依赖注入
- `web` Vue 3 界面；构建结果经 `embed.FS` 打进程序
