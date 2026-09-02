package httpserver

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/injoyai/ios/client"
	"github.com/injoyai/tdx"
)

// Option HTTP 服务配置选项
type Option func(*serverConfig)

// serverConfig HTTP 服务配置
type serverConfig struct {
	addr        string
	hosts       []string
	poolSize    int
	exHqHosts   []string
	exPoolSize  int
	macHosts    []string
	macPoolSize int
	options     []client.Option
}

// WithAddr 设置监听地址
func WithAddr(addr string) Option {
	return func(c *serverConfig) { c.addr = addr }
}

// WithHosts 设置标准行情服务器列表
func WithHosts(hosts ...string) Option {
	return func(c *serverConfig) { c.hosts = hosts }
}

// WithPoolSize 设置标准连接池大小
func WithPoolSize(n int) Option {
	return func(c *serverConfig) { c.poolSize = n }
}

// WithExHqHosts 设置扩展行情服务器列表,启用扩展行情 /ex/* 路由
func WithExHqHosts(hosts ...string) Option {
	return func(c *serverConfig) { c.exHqHosts = hosts }
}

// WithExPoolSize 设置扩展连接池大小
func WithExPoolSize(n int) Option {
	return func(c *serverConfig) { c.exPoolSize = n }
}

// WithMacHosts 设置 mac 方言服务器列表,启用 /mac/* 路由
func WithMacHosts(hosts ...string) Option {
	return func(c *serverConfig) { c.macHosts = hosts }
}

// WithMacPoolSize 设置 mac 连接池大小
func WithMacPoolSize(n int) Option {
	return func(c *serverConfig) { c.macPoolSize = n }
}

// WithOptions 设置通达信连接选项,如 tdx.WithDebug()、tdx.WithRedial()
func WithOptions(opts ...client.Option) Option {
	return func(c *serverConfig) {
		c.options = append(c.options, opts...)
	}
}

// Server HTTP 服务
type Server struct {
	pool    tdx.IPool
	exPool  tdx.IPool
	macPool tdx.IPool
	server  *http.Server
}

// New 创建并初始化 HTTP 服务
func New(opts ...Option) (*Server, error) {
	cfg := &serverConfig{
		addr:     ":8080",
		hosts:    tdx.Hosts,
		poolSize: 1,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	pool, err := tdx.NewPool(func() (*tdx.Client, error) {
		return tdx.DialHostsRange(cfg.hosts, cfg.options...)
	}, cfg.poolSize)
	if err != nil {
		return nil, err
	}

	// 初始化 DefaultCodes(ETF/可转债等非股票行情需要查 Decimal 修正价格, 见 Client.GetQuote)
	if tdx.DefaultCodes == nil {
		codesClient, err := tdx.DialHostsRange(cfg.hosts, cfg.options...)
		if err != nil {
			return nil, err
		}
		tdx.DefaultCodes, err = tdx.NewCodesSqlite(tdx.WithCodesClient(codesClient))
		if err != nil {
			return nil, err
		}
	}

	s := &Server{pool: pool}

	if len(cfg.exHqHosts) > 0 {
		if cfg.exPoolSize <= 0 {
			cfg.exPoolSize = 1
		}
		exPool, err := tdx.NewPool(func() (*tdx.Client, error) {
			return tdx.DialExHqHosts(cfg.exHqHosts, cfg.options...)
		}, cfg.exPoolSize)
		if err != nil {
			return nil, err
		}
		s.exPool = exPool
	}

	// mac 方言连接池(独立服务器池, 与标准池不互通)
	if len(cfg.macHosts) > 0 {
		if cfg.macPoolSize <= 0 {
			cfg.macPoolSize = 1
		}
		macPool, err := tdx.NewPool(func() (*tdx.Client, error) {
			return tdx.DialMacHosts(cfg.macHosts, cfg.options...)
		}, cfg.macPoolSize)
		if err != nil {
			return nil, err
		}
		s.macPool = macPool
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.server = &http.Server{
		Addr:              cfg.addr,
		Handler:           s.recoverMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
		// 不设 WriteTimeout: 大响应(如全量逐笔 ~500KB)在慢客户端上会被中途掐断,
		// 截断比慢更伤(客户端拿到非法 JSON); 慢由客户端自身超时控制。
	}
	return s, nil
}

// Default 使用默认配置创建 HTTP 服务(开启断线重连)
func Default(opts ...Option) (*Server, error) {
	opts = append([]Option{WithOptions(tdx.WithRedial())}, opts...)
	return New(opts...)
}

// Run 启动 HTTP 服务
func (s *Server) Run() error {
	return s.server.ListenAndServe()
}

// Close 关闭 HTTP 服务
func (s *Server) Close() error {
	return s.server.Close()
}

// registerRoutes 注册所有路由
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// 健康检查
	mux.HandleFunc("GET /", s.handleHealth)

	// 代码/数量
	mux.HandleFunc("GET /count", s.handleCount)
	mux.HandleFunc("GET /code", s.handleCode)
	mux.HandleFunc("GET /code/all", s.handleCodeAll)
	mux.HandleFunc("GET /code/stocks", s.handleStockCodeAll)
	mux.HandleFunc("GET /code/etfs", s.handleETFCodeAll)
	mux.HandleFunc("GET /code/indexes", s.handleIndexCodeAll)

	// 行情/财务
	mux.HandleFunc("GET /quote", s.handleQuote)
	mux.HandleFunc("GET /call_auction", s.handleCallAuction)
	mux.HandleFunc("GET /gbbq", s.handleGbbq)
	mux.HandleFunc("GET /finance", s.handleFinanceInfo)
	mux.HandleFunc("GET /company/category", s.handleCompanyCategory)
	mux.HandleFunc("GET /company/content", s.handleCompanyContent)

	// 分时/成交
	mux.HandleFunc("GET /minute", s.handleMinute)
	mux.HandleFunc("GET /minute/history", s.handleHistoryMinute)
	mux.HandleFunc("GET /trade", s.handleTrade)
	mux.HandleFunc("GET /trade/all", s.handleTradeAll)
	mux.HandleFunc("GET /trade/history", s.handleHistoryTrade)
	mux.HandleFunc("GET /trade/history/day", s.handleHistoryTradeDay)

	// K线(股票)
	mux.HandleFunc("GET /kline", s.handleKline)
	mux.HandleFunc("GET /kline/all", s.handleKlineAll)
	mux.HandleFunc("GET /kline/minute", s.handleKlineMinute)
	mux.HandleFunc("GET /kline/minute/all", s.handleKlineMinuteAll)
	mux.HandleFunc("GET /kline/5minute", s.handleKline5Minute)
	mux.HandleFunc("GET /kline/5minute/all", s.handleKline5MinuteAll)
	mux.HandleFunc("GET /kline/15minute", s.handleKline15Minute)
	mux.HandleFunc("GET /kline/15minute/all", s.handleKline15MinuteAll)
	mux.HandleFunc("GET /kline/30minute", s.handleKline30Minute)
	mux.HandleFunc("GET /kline/30minute/all", s.handleKline30MinuteAll)
	mux.HandleFunc("GET /kline/60minute", s.handleKline60Minute)
	mux.HandleFunc("GET /kline/60minute/all", s.handleKline60MinuteAll)
	mux.HandleFunc("GET /kline/day", s.handleKlineDay)
	mux.HandleFunc("GET /kline/day/all", s.handleKlineDayAll)
	mux.HandleFunc("GET /kline/week", s.handleKlineWeek)
	mux.HandleFunc("GET /kline/week/all", s.handleKlineWeekAll)
	mux.HandleFunc("GET /kline/month", s.handleKlineMonth)
	mux.HandleFunc("GET /kline/month/all", s.handleKlineMonthAll)
	mux.HandleFunc("GET /kline/quarter", s.handleKlineQuarter)
	mux.HandleFunc("GET /kline/quarter/all", s.handleKlineQuarterAll)
	mux.HandleFunc("GET /kline/year", s.handleKlineYear)
	mux.HandleFunc("GET /kline/year/all", s.handleKlineYearAll)

	// 指数K线
	mux.HandleFunc("GET /index", s.handleIndex)
	mux.HandleFunc("GET /index/all", s.handleIndexAll)
	mux.HandleFunc("GET /index/minute", s.handleIndexMinute)
	mux.HandleFunc("GET /index/5minute", s.handleIndex5Minute)
	mux.HandleFunc("GET /index/15minute", s.handleIndex15Minute)
	mux.HandleFunc("GET /index/30minute", s.handleIndex30Minute)
	mux.HandleFunc("GET /index/60minute", s.handleIndex60Minute)
	mux.HandleFunc("GET /index/day", s.handleIndexDay)
	mux.HandleFunc("GET /index/day/all", s.handleIndexDayAll)
	mux.HandleFunc("GET /index/week/all", s.handleIndexWeekAll)
	mux.HandleFunc("GET /index/month/all", s.handleIndexMonthAll)
	mux.HandleFunc("GET /index/quarter/all", s.handleIndexQuarterAll)
	mux.HandleFunc("GET /index/year/all", s.handleIndexYearAll)

	// 板块/报表
	mux.HandleFunc("GET /block/data", s.handleBlockData)
	mux.HandleFunc("GET /block/data/index", s.handleBlockDataWithIndex)
	mux.HandleFunc("GET /block/file", s.handleBlockFileRaw)
	mux.HandleFunc("GET /report/file", s.handleReportFile)
	mux.HandleFunc("GET /zhb/files", s.handleZHBFiles)
	mux.HandleFunc("GET /tdx/zs", s.handleTdxZs)
	mux.HandleFunc("GET /tdx/bk", s.handleTdxBk)
	mux.HandleFunc("GET /tdx/stat", s.handleTdxStat)
	mux.HandleFunc("GET /tdx/stat2", s.handleTdxStat2)
	mux.HandleFunc("GET /tdx/xgsg", s.handleTdxXgsg)
	mux.HandleFunc("GET /tdx/hy", s.handleTdxHy)
	mux.HandleFunc("GET /spblock", s.handleSpBlock)

	// 接口文档(规范内嵌于二进制, 无需外部文件)
	mux.HandleFunc("GET /openapi.yaml", handleOpenAPI)
	mux.HandleFunc("GET /docs", handleDocs)

	// 扩展行情
	if s.exPool != nil {
		mux.HandleFunc("GET /ex/markets", s.handleExMarkets)
		mux.HandleFunc("GET /ex/count", s.handleExCount)
		mux.HandleFunc("GET /ex/instruments", s.handleExInstruments)
		mux.HandleFunc("GET /ex/quote", s.handleExQuote)
		mux.HandleFunc("GET /ex/quote_list", s.handleExQuoteList)
		mux.HandleFunc("GET /ex/bars", s.handleExBars)
		mux.HandleFunc("GET /ex/minute", s.handleExMinute)
		mux.HandleFunc("GET /ex/minute/hist", s.handleExHistMinute)
		mux.HandleFunc("GET /ex/trade", s.handleExTrade)
		mux.HandleFunc("GET /ex/trade/hist", s.handleExHistTrade)
		mux.HandleFunc("GET /ex/bars/range", s.handleExBarsRange)
	}

	// mac 方言(独立服务器池; 主力净流入/秒级逐笔等标准协议没有的数据)
	if s.macPool != nil {
		mux.HandleFunc("GET /mac/quote", s.handleMacQuote)
		mux.HandleFunc("GET /mac/trade", s.handleMacTrade)
		mux.HandleFunc("GET /mac/trade/all", s.handleMacTradeAll)
		mux.HandleFunc("GET /mac/trade/history", s.handleMacHistoryTrade)
		mux.HandleFunc("GET /mac/capital_flow", s.handleMacCapitalFlow)
		mux.HandleFunc("GET /mac/belong_boards", s.handleMacBelongBoards)
		mux.HandleFunc("GET /mac/board_members", s.handleMacBoardMembers)
		mux.HandleFunc("GET /mac/server_session", s.handleMacServerSession)
		mux.HandleFunc("GET /mac/kline_count", s.handleMacKlineCount)
	}
}

// recoverMiddleware panic 兜底: handler 异常时返回干净的 500 JSON,
// 而不是让 net/http 直接掐断连接(客户端表现为响应截断/非法 JSON)
func (s *Server) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if e := recover(); e != nil {
				log.Printf("[http] panic %s %s: %v", r.Method, r.URL, e)
				respondErr(w, http.StatusInternalServerError, fmt.Sprintf("服务器内部错误: %v", e))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// handleHealth 健康检查
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondOK(w, map[string]string{"status": "running"})
}
