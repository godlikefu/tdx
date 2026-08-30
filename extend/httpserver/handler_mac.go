package httpserver

import (
	"net/http"
	"strings"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

// mac 方言接口: 主力净流入/秒级逐笔/板块体系等标准协议没有的数据。
// 需通过 WithMacHosts(...) 启用 mac 连接池, 否则路由不注册。

// requireMacPool 校验 mac 池可用
func (s *Server) requireMacPool(w http.ResponseWriter) bool {
	if s.macPool == nil {
		respondErr(w, http.StatusServiceUnavailable, "mac 服务未启用, 请配置 WithMacHosts(...)")
		return false
	}
	return true
}

// handleMacQuote GET /mac/quote?codes=sh601872,sz000001
// 批量自定义字段报价(实时): 主力净流入/内外盘/涨跌停价/PE/盘后量等, 单次≤80只
func (s *Server) handleMacQuote(w http.ResponseWriter, r *http.Request) {
	if !s.requireMacPool(w) {
		return
	}
	codesStr, err := queryStr(r, "codes")
	if err != nil {
		respondErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var quotes []*protocol.MacQuote
	err = s.macPool.Do(func(c *tdx.Client) error {
		var e error
		quotes, e = c.GetMacQuote(strings.Split(codesStr, ",")...)
		return e
	})
	if err != nil {
		respondErr(w, http.StatusBadGateway, err.Error())
		return
	}
	respondOK(w, quotes)
}

// handleMacTrade GET /mac/trade?code=sh601872&start=0&count=100
// 秒级逐笔成交(含成交笔数/盘后固定价), start 从最新端往回
func (s *Server) handleMacTrade(w http.ResponseWriter, r *http.Request) {
	if !s.requireMacPool(w) {
		return
	}
	code, err := queryStr(r, "code")
	if err != nil {
		respondErr(w, http.StatusBadRequest, err.Error())
		return
	}
	start := queryUint16Default(r, "start", 0)
	count := queryUint16Default(r, "count", 100)
	var resp *protocol.MacTradeResp
	err = s.macPool.Do(func(c *tdx.Client) error {
		var e error
		resp, e = c.GetMacTrade(code, uint32(start), count)
		return e
	})
	if err != nil {
		respondErr(w, http.StatusBadGateway, err.Error())
		return
	}
	respondOK(w, resp)
}

// handleMacTradeAll GET /mac/trade/all?code=sh601872
// 当日全量秒级逐笔(并发分页, 时间正序)
func (s *Server) handleMacTradeAll(w http.ResponseWriter, r *http.Request) {
	if !s.requireMacPool(w) {
		return
	}
	code, err := queryStr(r, "code")
	if err != nil {
		respondErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var resp *protocol.MacTradeResp
	err = s.macPool.Do(func(c *tdx.Client) error {
		var e error
		resp, e = c.GetMacTradeAll(code)
		return e
	})
	if err != nil {
		respondErr(w, http.StatusBadGateway, err.Error())
		return
	}
	respondOK(w, resp)
}

// handleMacHistoryTrade GET /mac/trade/history?code=sh601872&date=20260828&start=0&count=100
// 指定日期秒级逐笔
func (s *Server) handleMacHistoryTrade(w http.ResponseWriter, r *http.Request) {
	if !s.requireMacPool(w) {
		return
	}
	code, err := queryStr(r, "code")
	if err != nil {
		respondErr(w, http.StatusBadRequest, err.Error())
		return
	}
	date, err := queryStr(r, "date")
	if err != nil {
		respondErr(w, http.StatusBadRequest, err.Error())
		return
	}
	start := queryUint16Default(r, "start", 0)
	count := queryUint16Default(r, "count", 100)
	var resp *protocol.MacTradeResp
	err = s.macPool.Do(func(c *tdx.Client) error {
		var e error
		resp, e = c.GetMacHistoryTrade(date, code, uint32(start), count)
		return e
	})
	if err != nil {
		respondErr(w, http.StatusBadGateway, err.Error())
		return
	}
	respondOK(w, resp)
}

// handleMacCapitalFlow GET /mac/capital_flow?code=sh601872
// 个股资金流向(主力/散户净额, 通达信口径)
func (s *Server) handleMacCapitalFlow(w http.ResponseWriter, r *http.Request) {
	if !s.requireMacPool(w) {
		return
	}
	code, err := queryStr(r, "code")
	if err != nil {
		respondErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var resp *protocol.MacCapitalFlow
	err = s.macPool.Do(func(c *tdx.Client) error {
		var e error
		resp, e = c.GetMacCapitalFlow(code)
		return e
	})
	if err != nil {
		respondErr(w, http.StatusBadGateway, err.Error())
		return
	}
	respondOK(w, resp)
}

// handleMacBelongBoards GET /mac/belong_boards?code=sh601872
// 个股所属板块(概念/行业/地域)
func (s *Server) handleMacBelongBoards(w http.ResponseWriter, r *http.Request) {
	if !s.requireMacPool(w) {
		return
	}
	code, err := queryStr(r, "code")
	if err != nil {
		respondErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var resp []*protocol.MacBelongBoard
	err = s.macPool.Do(func(c *tdx.Client) error {
		var e error
		resp, e = c.GetMacBelongBoards(code)
		return e
	})
	if err != nil {
		respondErr(w, http.StatusBadGateway, err.Error())
		return
	}
	respondOK(w, resp)
}

// handleMacBoardMembers GET /mac/board_members?board=880216&start=0&count=10
// 板块成分报价(按涨幅降序, 显示代码自动转协议代码)
func (s *Server) handleMacBoardMembers(w http.ResponseWriter, r *http.Request) {
	if !s.requireMacPool(w) {
		return
	}
	board, err := queryStr(r, "board")
	if err != nil {
		respondErr(w, http.StatusBadRequest, err.Error())
		return
	}
	start := queryUint16Default(r, "start", 0)
	count := queryUint16Default(r, "count", 10)
	var resp []*protocol.MacQuote
	err = s.macPool.Do(func(c *tdx.Client) error {
		var e error
		resp, e = c.GetMacBoardMembers(board, start, count)
		return e
	})
	if err != nil {
		respondErr(w, http.StatusBadGateway, err.Error())
		return
	}
	respondOK(w, resp)
}

// handleMacServerSession GET /mac/server_session
// 服务器交易时段与交易日历
func (s *Server) handleMacServerSession(w http.ResponseWriter, r *http.Request) {
	if !s.requireMacPool(w) {
		return
	}
	var resp *protocol.MacServerSession
	err := s.macPool.Do(func(c *tdx.Client) error {
		var e error
		resp, e = c.GetMacServerSession()
		return e
	})
	if err != nil {
		respondErr(w, http.StatusBadGateway, err.Error())
		return
	}
	respondOK(w, resp)
}

// handleMacKlineCount GET /mac/kline_count
// K线数据总量(可兼作 mac 服务探活)
func (s *Server) handleMacKlineCount(w http.ResponseWriter, r *http.Request) {
	if !s.requireMacPool(w) {
		return
	}
	var resp *protocol.MacKlineCount
	err := s.macPool.Do(func(c *tdx.Client) error {
		var e error
		resp, e = c.GetMacKlineCount()
		return e
	})
	if err != nil {
		respondErr(w, http.StatusBadGateway, err.Error())
		return
	}
	respondOK(w, resp)
}
