package main

// mac HTTP API 服务示例: 标准行情 + mac 方言双池, 一条命令起服务
//
//	curl "http://127.0.0.1:8080/quote?codes=sh601872"        标准五档行情
//	curl "http://127.0.0.1:8080/mac/quote?codes=sh601872"    mac 增强行情(主力净流入/五档盘口/委比)
//	curl "http://127.0.0.1:8080/mac/trade/all?code=sh601872" 秒级逐笔
//	curl "http://127.0.0.1:8080/mac/capital_flow?code=sh601872"
//	全部路由见 extend/httpserver/README.md
import (
	"log"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/extend/httpserver"
)

func main() {
	s, err := httpserver.New(
		httpserver.WithAddr(":8080"),
		httpserver.WithPoolSize(1),               //标准行情连接池
		httpserver.WithMacHosts(tdx.MacHosts...), //启用 /mac/* 路由(独立 mac 服务器池)
		httpserver.WithMacPoolSize(1),
		httpserver.WithOptions(tdx.WithRedial()), //断线重连
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("HTTP API 已启动: http://127.0.0.1:8080 (含 /mac/* 路由)")
	log.Fatal(s.Run())
}
