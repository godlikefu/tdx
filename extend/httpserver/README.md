# tdx HTTP Server

> **OpenAPI 规范**: [`openapi.yaml`](openapi.yaml)(OpenAPI 3.0, 86 条路由, 与代码机械同步校验), 已通过 `go:embed` 内嵌进服务二进制:
> - `GET /docs` — 内置 Swagger UI 在线预览/调试(浏览器需可访问 unpkg.com CDN)
> - `GET /openapi.yaml` — 下载规范文件, 离线导入 [Apifox](https://apifox.com)/Postman/Insomnia 或拖入 [Swagger Editor](https://editor.swagger.io)

将通达信(tdx)行情数据通过 HTTP API 对外开放。本包基于 `tdx.Client` 实现,以 RESTful GET 接口暴露股票、指数、扩展行情等数据。

## mac 方言接口 (/mac/*)

mac 服务器池与标准池**独立**, 需显式配置 `WithMacHosts(...)` 启用(未配置时这些路由不注册, 返回 404):

```go
s, _ := httpserver.New(
    httpserver.WithAddr(":8080"),
    httpserver.WithMacHosts(tdx.MacHosts...), // 启用 /mac/* 路由
    httpserver.WithMacPoolSize(1),
)
```

| 路由 | 参数 | 说明 |
|---|---|---|
| GET /mac/quote | codes=sh601872,sz000001 | 批量自定义字段报价(主力净流入/内外盘/涨跌停价/PE/盘后量/**五档盘口/委比**等 46 字段, ≤80只) |
| GET /mac/trade | code, start=0, count=100 | 秒级逐笔成交(含笔数/盘后固定价), start 从最新端往回 |
| GET /mac/trade/all | code | 当日全量秒级逐笔(并发分页, 时间正序) |
| GET /mac/trade/history | code, date=20260828, start, count | 指定日期秒级逐笔 |
| GET /mac/capital_flow | code | 资金流向(主力/散户净额, 通达信口径) |
| GET /mac/belong_boards | code | 个股所属板块(概念/行业/地域) |
| GET /mac/board_members | board=880216, start, count | 板块成分报价(按涨幅降序) |
| GET /mac/server_session | - | 服务器交易时段与交易日历 |
| GET /mac/kline_count | - | K线数据总量(可兼作探活) |

### mac 接口请求示例

以下均为实测响应(数据日 2026-08-28 收盘快照; 价格字段单位=厘(元×1000), 成交额=元):

```bash
# 批量自定义字段报价: 主力净流入/内外盘/涨跌停价/PE/五档盘口/委比 (≤80只/次)
curl "http://127.0.0.1:8080/mac/quote?codes=sh601872,sz000001"
{
  "code": 0, "msg": "ok",
  "data": [{
    "Market": 1, "Code": "601872", "Name": "招商轮船",
    "PreClose": 18620, "Open": 19000, "High": 19300, "Low": 18820,
    "Price": 18980, "AvgPrice": 19106,
    "BuyPriceLimit": 20480, "SellPriceLimit": 16760,
    "Volume": 864362, "Amount": 1651491328,
    "InsideVolume": 427871, "OutsideVolume": 436491,
    "VolRatio": 0.71, "Turnover": 1.07, "PE": 13.86,
    "LastVolume": 6432, "AfterHoursVol": 38100,
    "MainNetAmount": -104935744, "MainNetRatio": -0.22, "EntrustRatio": 30.16,
    "Bids": [ { "Price": 18980, "Vol": 364 }, { "Price": 18970, "Vol": 736 },
              { "Price": 18960, "Vol": 840 }, { "Price": 18950, "Vol": 1036 },
              { "Price": 18940, "Vol": 761 } ],
    "Asks": [ { "Price": 18990, "Vol": 302 }, { "Price": 19000, "Vol": 970 },
              { "Price": 19010, "Vol": 138 }, { "Price": 19020, "Vol": 385 },
              { "Price": 19030, "Vol": 210 } ]
  }]
}

# 当日全量秒级逐笔(含集合竞价/盘后固定价, Time 日期部分为客户端当日戳)
curl "http://127.0.0.1:8080/mac/trade/all?code=sh601872"
{
  "code": 0, "msg": "ok",
  "data": { "Count": 4597, "Total": 4597, "List": [
    { "Time": "2026-08-30T09:25:01+08:00", "Price": 19000, "Volume": 7907, "TradeCount": 626, "Status": 2 },
    { "Time": "2026-08-30T09:30:01+08:00", "Price": 18990, "Volume": 493,  "TradeCount": 36,  "Status": 1 }
  ]}
}
# Status: 0=买盘 1=卖盘 2=中性(竞价) 5=盘后固定价

# 分页取逐笔: start 从最新端往回
curl "http://127.0.0.1:8080/mac/trade?code=sh601872&start=0&count=100"
# 指定日期逐笔
curl "http://127.0.0.1:8080/mac/trade/history?code=sh601872&date=20260828&start=0&count=100"

# 资金流向(主力/散户净额, 通达信口径; FiveDay=[买5日,卖5日,超大单,大单,中单,小单] 口径待考)
curl "http://127.0.0.1:8080/mac/capital_flow?code=sh601872"
{
  "code": 0, "msg": "ok",
  "data": { "MainIn": 505323328, "MainOut": 610259072, "MainNet": -104935744,
            "SmallIn": 1146102016, "SmallOut": 1041166144, "SmallNet": 104935872,
            "FiveDay": [4646344192, 4910187008, -615941120, -60462960, -118721280, 795125120] }
}

# 所属板块
curl "http://127.0.0.1:8080/mac/belong_boards?code=sh601872"
{ "code": 0, "msg": "ok", "data": [
  { "BoardType": 3, "Market": 1, "BoardCode": "880216", "BoardName": "上海板块", "Close": 1866.63, "PreClose": 1879.19 },
  { "BoardType": 5, "Market": 1, "BoardCode": "880578", "BoardName": "专项贷款", "Close": 1513.22, "PreClose": 1507.48 }
]}

# 板块成分(按涨幅降序; 板块显示代码自动转协议代码 880216→20216)
curl "http://127.0.0.1:8080/mac/board_members?board=880216&start=0&count=10"

# 服务器交易时段/交易日历
curl "http://127.0.0.1:8080/mac/server_session"
{ "code": 0, "msg": "ok", "data": {
  "Today": "2026-08-30T00:00:00+08:00", "LastTradingDay": "2026-08-28T00:00:00+08:00",
  "Sessions1": [ { "Open": 570, "Close": 690 }, { "Open": 780, "Close": 900 },
                 { "Open": 0, "Close": 0 }, { "Open": 0, "Close": 0 } ],
  "Sessions2": [ ... ], "MarketParam1": 0, "MarketParam2": 0
}}
# Open/Close 为分钟数(570=09:30, 900=15:00)

# K线总量(可兼作 mac 服务探活)
curl "http://127.0.0.1:8080/mac/kline_count"
{ "code": 0, "msg": "ok", "data": { "Total": 0, "Returned": 800 } }
```

`/mac/quote` 字段说明: 价格均为厘(除以 1000 得元); Volume/InsideVolume/OutsideVolume 及 Bids/Asks 的 Vol 单位=手; MainNetAmount/Amount 单位=元; 内盘+外盘=总量已实测恒等。

## 快速开始

方式一(推荐): 命令行启动器

```bash
go run ./cmd/tdx-api              # :8080, 标准池+mac池
go run ./cmd/tdx-api -addr :9090 -pool 4 -mac=false -ex -debug
# 参数: -addr 监听地址 | -pool 标准池大小 | -mac 启用/mac路由
#       -mac-pool mac池大小 | -ex 启用/ex路由 | -debug 协议调试日志
```

方式二: 代码内嵌

```go
package main

import (
	"log"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/extend/httpserver"
)

func main() {
	// 方式一: 默认配置(开启断线重连)
	s, err := httpserver.Default()
	if err != nil {
		log.Fatal(err)
	}

	// 方式二: 自定义配置
	s, err = httpserver.New(
		httpserver.WithAddr(":8080"),
		httpserver.WithPoolSize(2),
		httpserver.WithExHqHosts(tdx.ExHosts...), // 可选,启用扩展行情 /ex/* 路由
		httpserver.WithOptions(tdx.WithRedial()),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("服务启动,监听 :8080")
	if err := s.Run(); err != nil {
		log.Fatal(err)
	}
}
```

## 配置选项

使用函数式选项(Functional Options)配置服务:

| 选项函数 | 说明 | 默认值 |
| --- | --- | --- |
| `WithAddr(addr)` | HTTP 监听地址 | `":8080"` |
| `WithHosts(hosts...)` | 标准行情服务器列表 | `tdx.Hosts` |
| `WithPoolSize(n)` | 标准连接池大小 | `1` |
| `WithExHqHosts(hosts...)` | 扩展行情服务器列表,为空则不启用扩展行情 | 无 |
| `WithExPoolSize(n)` | 扩展连接池大小 | `1` |
| `WithMacHosts(hosts...)` | mac 方言服务器列表,为空则不注册 `/mac/*` 路由 | 无 |
| `WithMacPoolSize(n)` | mac 连接池大小(电子表格等多并发刷新建议 2~4) | `1` |
| `WithOptions(opts...)` | 通达信连接选项,如 `tdx.WithDebug()`、`tdx.WithRedial()` | 无 |

> `Default()` 会自动添加 `tdx.WithRedial()` 断线重连选项。

## 响应格式

所有接口统一返回如下 JSON 结构:

**成功:**

```json
{
  "code": 0,
  "msg": "ok",
  "data": { ... }
}
```

**错误:**

```json
{
  "code": 1,
  "msg": "错误信息",
  "data": null
}
```

## API 路由

### 健康检查

| 路径 | 参数 | 说明 |
| --- | --- | --- |
| `GET /` | 无 | 健康检查,返回服务状态 |

### 代码/数量

| 路径 | 参数 | 说明 |
| --- | --- | --- |
| `GET /count` | `exchange` | 获取指定交易所的证券数量 |
| `GET /code` | `exchange`, `start` | 获取指定交易所的证券代码(分页) |
| `GET /code/all` | `exchange` | 获取指定交易所的全部证券代码 |
| `GET /code/stocks` | 无 | 获取全部股票代码 |
| `GET /code/etfs` | 无 | 获取全部 ETF 代码 |
| `GET /code/indexes` | 无 | 获取全部指数代码 |

### 行情/财务

| 路径 | 参数 | 说明 |
| --- | --- | --- |
| `GET /quote` | `codes` | 获取实时行情报价(支持多个代码) |
| `GET /call_auction` | `code` | 获取集合竞价数据 |
| `GET /gbbq` | `code` | 获取除权除息(股本变更)数据 |
| `GET /finance` | `exchange`, `code` | 获取财务信息 |
| `GET /company/category` | `exchange`, `code` | 获取公司信息(F10)文件目录 |
| `GET /company/content` | `exchange`, `code`, `filename`, `start`, `length` | 获取公司信息(F10)文件内容 |

### 分时/成交

| 路径 | 参数 | 说明 |
| --- | --- | --- |
| `GET /minute` | `code` | 获取当日分时数据 |
| `GET /minute/history` | `date`, `code` | 获取历史分时数据 |
| `GET /trade` | `code`, `start`, `count` | 获取当日分笔成交明细(分页) |
| `GET /trade/all` | `code` | 获取当日全部分笔成交明细 |
| `GET /trade/history` | `date`, `code`, `start`, `count` | 获取历史分笔成交明细(分页) |
| `GET /trade/history/day` | `date`, `code` | 获取指定日期全部分笔成交明细 |

### K线(股票)

| 路径 | 参数 | 说明 |
| --- | --- | --- |
| `GET /kline` | `type`, `code`, `start`, `count` | 获取指定类型的 K 线(分页) |
| `GET /kline/all` | `type`, `code` | 获取指定类型的全部 K 线 |
| `GET /kline/minute` | `code`, `start`, `count` | 获取 1 分钟 K 线(分页) |
| `GET /kline/minute/all` | `code` | 获取全部 1 分钟 K 线 |
| `GET /kline/5minute` | `code`, `start`, `count` | 获取 5 分钟 K 线(分页) |
| `GET /kline/5minute/all` | `code` | 获取全部 5 分钟 K 线 |
| `GET /kline/15minute` | `code`, `start`, `count` | 获取 15 分钟 K 线(分页) |
| `GET /kline/15minute/all` | `code` | 获取全部 15 分钟 K 线 |
| `GET /kline/30minute` | `code`, `start`, `count` | 获取 30 分钟 K 线(分页) |
| `GET /kline/30minute/all` | `code` | 获取全部 30 分钟 K 线 |
| `GET /kline/60minute` | `code`, `start`, `count` | 获取 60 分钟 K 线(分页) |
| `GET /kline/60minute/all` | `code` | 获取全部 60 分钟 K 线 |
| `GET /kline/day` | `code`, `start`, `count` | 获取日 K 线(分页) |
| `GET /kline/day/all` | `code` | 获取全部日 K 线 |
| `GET /kline/week` | `code`, `start`, `count` | 获取周 K 线(分页) |
| `GET /kline/week/all` | `code` | 获取全部周 K 线 |
| `GET /kline/month` | `code`, `start`, `count` | 获取月 K 线(分页) |
| `GET /kline/month/all` | `code` | 获取全部月 K 线 |
| `GET /kline/quarter` | `code`, `start`, `count` | 获取季 K 线(分页) |
| `GET /kline/quarter/all` | `code` | 获取全部季 K 线 |
| `GET /kline/year` | `code`, `start`, `count` | 获取年 K 线(分页) |
| `GET /kline/year/all` | `code` | 获取全部年 K 线 |

### 指数K线

| 路径 | 参数 | 说明 |
| --- | --- | --- |
| `GET /index` | `type`, `code`, `start`, `count` | 获取指定类型的指数 K 线(分页) |
| `GET /index/all` | `type`, `code` | 获取指定类型的全部指数 K 线 |
| `GET /index/minute` | `code`, `start`, `count` | 获取指数 1 分钟 K 线(分页) |
| `GET /index/5minute` | `code`, `start`, `count` | 获取指数 5 分钟 K 线(分页) |
| `GET /index/15minute` | `code`, `start`, `count` | 获取指数 15 分钟 K 线(分页) |
| `GET /index/30minute` | `code`, `start`, `count` | 获取指数 30 分钟 K 线(分页) |
| `GET /index/60minute` | `code`, `start`, `count` | 获取指数 60 分钟 K 线(分页) |
| `GET /index/day` | `code`, `start`, `count` | 获取指数日 K 线(分页) |
| `GET /index/day/all` | `code` | 获取全部指数日 K 线 |
| `GET /index/week/all` | `code` | 获取全部指数周 K 线 |
| `GET /index/month/all` | `code` | 获取全部指数月 K 线 |
| `GET /index/quarter/all` | `code` | 获取全部指数季 K 线 |
| `GET /index/year/all` | `code` | 获取全部指数年 K 线 |

### 板块/报表

| 路径 | 参数 | 说明 |
| --- | --- | --- |
| `GET /block/data` | `file` | 获取板块数据(解析后) |
| `GET /block/data/index` | `file` | 获取带索引的板块数据 |
| `GET /block/file` | `file` | 获取板块原始文件内容 |
| `GET /report/file` | `file` | 获取报表文件内容 |
| `GET /zhb/files` | 无 | 获取 ZHB 文件列表 |
| `GET /tdx/zs` | 无 | 获取通达信指数信息 |
| `GET /tdx/bk` | 无 | 获取通达信板块信息 |
| `GET /tdx/stat` | 无 | 获取通达信统计信息 |
| `GET /tdx/stat2` | 无 | 获取通达信统计信息(二) |
| `GET /tdx/xgsg` | 无 | 获取新股申购信息 |
| `GET /tdx/hy` | 无 | 获取通达信行业信息 |
| `GET /spblock` | 无 | 获取特殊板块信息 |

### 扩展行情

> 需要通过 `WithExHqHosts()` 选项启用,否则返回 404。

| 路径 | 参数 | 说明 |
| --- | --- | --- |
| `GET /ex/markets` | 无 | 获取扩展行情市场列表 |
| `GET /ex/count` | 无 | 获取扩展行情证券数量 |
| `GET /ex/instruments` | `start`, `count` | 获取扩展行情证券列表(分页) |
| `GET /ex/quote` | `market`, `code` | 获取扩展行情实时报价 |
| `GET /ex/quote_list` | `market`, `category`, `start`, `count` | 获取扩展行情报价列表(分页) |
| `GET /ex/bars` | `category`, `market`, `code`, `start`, `count` | 获取扩展行情 K 线(分页) |
| `GET /ex/minute` | `market`, `code` | 获取扩展行情分时数据 |
| `GET /ex/minute/hist` | `market`, `code`, `date` | 获取扩展行情历史分时数据 |
| `GET /ex/trade` | `market`, `code`, `start`, `count` | 获取扩展行情分笔成交(分页) |
| `GET /ex/trade/hist` | `market`, `code`, `date`, `start`, `count` | 获取扩展行情历史分笔成交(分页) |
| `GET /ex/bars/range` | `market`, `code`, `date`, `date2` | 获取扩展行情指定日期区间 K 线 |

## 参数说明

| 参数 | 说明 | 示例 |
| --- | --- | --- |
| `exchange` | 交易所代码,可选 `sh`(上海)、`sz`(深圳)、`bj`(北京) | `sh` |
| `code` | 证券代码,可带交易所前缀 | `600519` 或 `sh600519` |
| `codes` | 多个证券代码,逗号分隔 | `sz000001,sh600008` |
| `type` | K 线类型(数字),见下表 | `9` |
| `start` | 起始位置(数字) | `0` |
| `count` | 获取数量(数字) | `100` |
| `date` | 日期,格式 `YYYYMMDD`(如 `20240101`) | `20240101` |
| `market` | 扩展行情市场代码(数字) | `47` |
| `category` | 扩展行情类别(数字) | `1` |
| `file` | 板块/报表文件名 | `block_gn.dat` |
| `filename` | F10 公司信息文件名 | `300052.txt` |
| `length` | 长度(数字) | `5000` |
| `date2` | 结束日期,格式 `YYYYMMDD` | `20240601` |

**K 线类型(`type`)对照表:**

| 值 | 说明 |
| --- | --- |
| `0` | 5 分钟 |
| `1` | 15 分钟 |
| `2` | 30 分钟 |
| `3` | 60 分钟 |
| `4` | 日K(变体,数值需除以 100) |
| `5` | 周 |
| `6` | 月 |
| `7` | 1 分钟 |
| `8` | 1 分钟(变体) |
| `9` | 日 |
| `10` | 季 |
| `11` | 年 |

## 使用示例

**获取报价:**

```bash
curl "http://localhost:8080/quote?codes=sz000001,sh600519"
```

**获取日 K 线:**

```bash
# 使用通用接口,指定 type=9(日)
curl "http://localhost:8080/kline?type=9&code=600519&start=0&count=100"

# 使用专用接口
curl "http://localhost:8080/kline/day?code=600519&start=0&count=100"
```

**获取扩展行情:**

```bash
# 获取扩展行情市场列表
curl "http://localhost:8080/ex/markets"

# 获取扩展行情报价
curl "http://localhost:8080/ex/quote?market=47&code=600519"
```
