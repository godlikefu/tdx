package tdx

import (
	"os"
	"testing"
	"time"
)

// TestMacTradeLive 实连 mac 服务器验证秒级逐笔。需联网,默认跳过;设 TDX_MAC_LIVE=1 启用。
//
//	TDX_MAC_LIVE=1 go test . -run TestMacTradeLive -v
func TestMacTradeLive(t *testing.T) {
	if os.Getenv("TDX_MAC_LIVE") == "" {
		t.Skip("set TDX_MAC_LIVE=1 to run live mac-dialect test")
	}
	cli, err := DialMacDefault()
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer cli.Close()
	cli.SetTimeout(10 * time.Second)
	time.Sleep(800 * time.Millisecond)

	// 历史日期请求(数据稳定,可精确断言): 2026-08-28 sh601872 全天 4597 笔
	resp, err := cli.GetMacHistoryTrade("20260828", "sh601872", 0, 10)
	if err != nil {
		t.Fatalf("GetMacHistoryTrade: %v", err)
	}
	if resp.Total != 4597 {
		t.Errorf("Total = %d, 期望 4597", resp.Total)
	}
	if len(resp.List) != 10 {
		t.Fatalf("len(List) = %d, 期望 10", len(resp.List))
	}
	// start=0 为最新一端: 页内时间升序
	last := resp.List[len(resp.List)-1]
	t.Logf("最新端首条: %s", resp.List[0])
	t.Logf("最新端末条: %s", last)
	if resp.List[0].Time.After(last.Time) {
		t.Errorf("页内应时间升序: 首条 %v 晚于末条 %v", resp.List[0].Time, last.Time)
	}

	// 全量分页
	all, err := cli.GetMacHistoryTradeAll("20260828", "sh601872")
	if err != nil {
		t.Fatalf("GetMacHistoryTradeAll: %v", err)
	}
	if len(all.List) != 4597 {
		t.Errorf("全量条数 = %d, 期望 4597", len(all.List))
	}
	if len(all.List) > 0 {
		first, end := all.List[0], all.List[len(all.List)-1]
		t.Logf("时间正序首条: %s", first)
		t.Logf("时间正序末条: %s", end)
		if !first.Time.Before(end.Time) {
			t.Errorf("全量应时间正序: 首条 %v 应早于末条 %v", first.Time, end.Time)
		}
		// 集合竞价汇总: 09:25:01 19.00元 7907手 626笔
		if first.Time.Format("15:04:05") != "09:25:01" || first.Price.Float64() != 19.00 ||
			first.Volume != 7907 || first.TradeCount != 626 {
			t.Errorf("集合竞价汇总不符: %s", first)
		}
	}

	// 最近交易日(不指定日期,ymd=0)
	recent, err := cli.GetMacTrade("sz000001", 0, 5)
	if err != nil {
		t.Fatalf("GetMacTrade: %v", err)
	}
	if len(recent.List) == 0 {
		t.Error("GetMacTrade 应有数据")
	}
	for _, v := range recent.List {
		t.Logf("sz000001: %s", v)
	}

	// 批量自定义字段报价(实时), 与标准 GetQuote 交叉验证
	quotes, err := cli.GetMacQuote("sh601872", "sz000001")
	if err != nil {
		t.Fatalf("GetMacQuote: %v", err)
	}
	if len(quotes) != 2 {
		t.Fatalf("GetMacQuote len = %d, 期望 2", len(quotes))
	}
	for _, q := range quotes {
		t.Logf("mac行情: %s", q)
		t.Logf("  Open=%.3f High=%.3f Low=%.3f PreClose=%.3f Vol=%d 内盘=%d 外盘=%d 现量=%d 盘后量=%.0f",
			q.Open.Float64(), q.High.Float64(), q.Low.Float64(), q.PreClose.Float64(),
			q.Volume, q.InsideVolume, q.OutsideVolume, q.LastVolume, q.AfterHoursVol)
		t.Logf("  涨停=%.3f 跌停=%.3f 均价=%.3f PE=%.2f 总股本=%.0f(万) 流通=%.0f(万) 主力净比=%.2f%%",
			q.BuyPriceLimit.Float64(), q.SellPriceLimit.Float64(), q.AvgPrice.Float64(),
			q.PE, q.TotalShares, q.FloatShares, q.MainNetRatio)
		if q.Name == "" || q.Price <= 0 || q.PreClose <= 0 {
			t.Errorf("行情数据异常: %s", q)
		}
		if q.BuyPriceLimit <= 0 || q.SellPriceLimit <= 0 {
			t.Errorf("涨跌停价异常: %s", q)
		}
	}
	// 内部一致性断言(mac 连接不支持标准 0x053E, 无法与 GetQuote 同连接交叉验证)
	for _, q := range quotes {
		if q.InsideVolume+q.OutsideVolume != q.Volume {
			t.Errorf("内盘+外盘(%d) != 总量(%d): %s", q.InsideVolume+q.OutsideVolume, q.Volume, q.Code)
		}
		// 均价×股数 ≈ 成交额(1% 容差)
		if q.Volume > 0 && q.Amount > 0 {
			implied := q.AvgPrice.Float64() * float64(q.Volume) * 100
			if d := (implied - q.Amount) / q.Amount; d > 0.01 || d < -0.01 {
				t.Errorf("均价×量与额不符: implied=%.0f amount=%.0f (%s)", implied, q.Amount, q.Code)
			}
		}
	}

	// 资金流向(0x1218 head=2): 与 0x122B 的主力净流入同源, 应接近
	flow, err := cli.GetMacCapitalFlow("sh601872")
	if err != nil {
		t.Fatalf("GetMacCapitalFlow: %v", err)
	}
	t.Logf("资金流向: %s 五日原始=%v (0x122B 主力净额=%.0f)", flow, flow.FiveDay, quotes[0].MainNetAmount)
	if flow.MainIn == 0 && flow.MainOut == 0 {
		t.Error("资金流向主入/主出不应全为 0")
	}

	// 所属板块(0x1218 head=1)
	boards, err := cli.GetMacBelongBoards("sh601872")
	if err != nil {
		t.Fatalf("GetMacBelongBoards: %v", err)
	}
	if len(boards) == 0 {
		t.Error("所属板块不应为空")
	}
	for i, b := range boards {
		if i >= 5 {
			break
		}
		t.Logf("所属板块: %s", b)
	}

	// 板块成分(0x122C, 按涨幅降序前 10): 用 belong_board 返回的真实板块号(上海板块 880216)
	boardCode := "881160"
	if len(boards) > 0 {
		boardCode = boards[0].BoardCode
	}
	members, err := cli.GetMacBoardMembers(boardCode, 0, 10)
	if err != nil {
		t.Fatalf("GetMacBoardMembers: %v", err)
	}
	if len(members) == 0 {
		t.Errorf("板块 %s 成分不应为空", boardCode)
	}
	for i, m := range members {
		t.Logf("板块成分[%d]: %s %s 现价=%.3f 量=%d 额=%.0f", i, m.Code, m.Name, m.Price.Float64(), m.Volume, m.Amount)
	}

	// 交易时段(0x120F)
	session, err := cli.GetMacServerSession()
	if err != nil {
		t.Fatalf("GetMacServerSession: %v", err)
	}
	t.Logf("交易时段: 今天=%s 最近交易日=%s 时段=%s,%s / %s,%s",
		session.Today.Format("2006-01-02"), session.LastTradingDay.Format("2006-01-02"),
		session.Sessions1[0], session.Sessions1[1], session.Sessions2[0], session.Sessions2[1])

	// K线总量(0x124A)
	cnt, err := cli.GetMacKlineCount()
	if err != nil {
		t.Fatalf("GetMacKlineCount: %v", err)
	}
	t.Logf("%s", cnt)

	// 远程文件(0x1215/17): 探测元信息(文件名不确定, 仅记录不判失败)
	info, err := cli.GetMacFileInfo("tdxzs.cfg")
	if err != nil {
		t.Logf("GetMacFileInfo(tdxzs.cfg): %v (文件不存在属正常)", err)
	} else {
		t.Logf("文件元信息: %+v", info)
	}
}
