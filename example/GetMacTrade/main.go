package main

import (
	"fmt"

	"github.com/injoyai/logs"
	"github.com/injoyai/tdx"
)

// mac 方言(通达信 MAC 版客户端协议): 秒级逐笔成交,含成交笔数与盘后固定价格成交。
// 服务器为独立 mac 池(端口同 7709),见 DialMacDefault。
func main() {
	c, err := tdx.DialMacDefault()
	logs.PanicErr(err)
	defer c.Close()

	// 最近交易时段全部逐笔(自动分页,时间正序)
	resp, err := c.GetMacTradeAll("sh601872")
	logs.PanicErr(err)
	fmt.Printf("共 %d 笔 (服务器总数 %d)\n", resp.Count, resp.Total)
	for i, v := range resp.List {
		if i >= 5 && i < len(resp.List)-5 {
			continue
		}
		fmt.Println(v)
	}

	// 指定日期逐笔(YYYYMMDD)
	hist, err := c.GetMacHistoryTrade("20260828", "sh601872", 0, 10)
	logs.PanicErr(err)
	fmt.Printf("\n20260828 共 %d 笔,最新端 10 笔:\n", hist.Total)
	for _, v := range hist.List {
		fmt.Println(v)
	}

	// 批量自定义字段报价(实时): 主力净流入/内外盘/涨跌停价/PE 等标准协议没有的字段
	quotes, err := c.GetMacQuote("sh601872", "sz000001")
	logs.PanicErr(err)
	fmt.Println("\nmac 批量行情:")
	for _, q := range quotes {
		fmt.Println(q)
	}

	// 资金流向 / 所属板块
	flow, err := c.GetMacCapitalFlow("sh601872")
	logs.PanicErr(err)
	fmt.Printf("\n资金流向: %s\n", flow)
	boards, err := c.GetMacBelongBoards("sh601872")
	logs.PanicErr(err)
	for i, b := range boards {
		if i >= 5 {
			break
		}
		fmt.Printf("所属板块: %s\n", b)
	}

	// 板块成分(按涨幅降序前 5)
	if len(boards) > 0 {
		members, err := c.GetMacBoardMembers(boards[0].BoardCode, 0, 5)
		logs.PanicErr(err)
		fmt.Printf("\n板块 %s 涨幅前5:\n", boards[0].BoardName)
		for _, m := range members {
			fmt.Printf("  %s %s 现价=%.2f\n", m.Code, m.Name, m.Price.Float64())
		}
	}

	// 交易时段 / K线总量
	session, err := c.GetMacServerSession()
	logs.PanicErr(err)
	fmt.Printf("\n交易时段: 今天=%s 最近交易日=%s\n", session.Today.Format("2006-01-02"), session.LastTradingDay.Format("2006-01-02"))
	cnt, err := c.GetMacKlineCount()
	logs.PanicErr(err)
	fmt.Println(cnt)
}
