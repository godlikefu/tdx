package main

// tdx HTTP API 服务启动器
//
// 一键启动标准行情 + mac 方言双池 HTTP 服务:
//
//	go run ./cmd/tdx-api                  # 默认 :8080, 标准池+mac池
//	go run ./cmd/tdx-api -addr :9090      # 指定端口
//	go run ./cmd/tdx-api -mac=false       # 仅标准行情
//	go run ./cmd/tdx-api -ex              # 启用扩展行情 /ex/* (期货/港股/美股)
//	go run ./cmd/tdx-api -pool 4 -mac-pool 2 -debug
//
// 路由文档: extend/httpserver/README.md
import (
	"flag"
	"log"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/extend/httpserver"
)

func main() {
	addr := flag.String("addr", ":8080", "监听地址")
	pool := flag.Int("pool", 1, "标准行情连接池大小")
	mac := flag.Bool("mac", true, "启用 mac 方言路由(/mac/*)")
	macPool := flag.Int("mac-pool", 1, "mac 连接池大小")
	ex := flag.Bool("ex", false, "启用扩展行情路由(/ex/*, 期货/港股/美股)")
	debug := flag.Bool("debug", false, "输出协议调试日志")
	flag.Parse()

	cfg := []httpserver.Option{
		httpserver.WithAddr(*addr),
		httpserver.WithPoolSize(*pool),
	}
	if *debug {
		cfg = append(cfg, httpserver.WithOptions(tdx.WithRedial(), tdx.WithDebug()))
	} else {
		cfg = append(cfg, httpserver.WithOptions(tdx.WithRedial()))
	}
	if *mac {
		cfg = append(cfg,
			httpserver.WithMacHosts(tdx.MacHosts...),
			httpserver.WithMacPoolSize(*macPool),
		)
	}
	if *ex {
		cfg = append(cfg,
			httpserver.WithExHqHosts(tdx.ExHosts...),
			httpserver.WithExPoolSize(1),
		)
	}

	s, err := httpserver.New(cfg...)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("HTTP API 已启动: http://127.0.0.1%s", *addr)
	log.Printf("标准行情: /quote /kline/day /trade/all /minute ... (mac=%v ex=%v)", *mac, *ex)
	if *mac {
		log.Printf("mac 方言: /mac/quote /mac/trade/all /mac/capital_flow /mac/belong_boards ...")
	}
	log.Fatal(s.Run())
}
