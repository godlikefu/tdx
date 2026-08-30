package tdx

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/injoyai/base/maps"
	"github.com/injoyai/base/maps/wait"
	"github.com/injoyai/ios"
	"github.com/injoyai/ios/client"
	"github.com/injoyai/ios/module/tcp"
	"github.com/injoyai/tdx/protocol"
)

// DialMacDefault 连接默认 mac 方言服务器(遍历 MacHosts)。
func DialMacDefault(op ...client.Option) (*Client, error) {
	op = append([]client.Option{WithRedial()}, op...)
	return DialMacHosts(MacHosts, op...)
}

// DialMac 连接指定 mac 方言服务器。
func DialMac(addr string, op ...client.Option) (*Client, error) {
	if !strings.Contains(addr, ":") {
		addr += ":" + MacPort
	}
	return dialMacWith(tcp.NewDial(addr), op...)
}

// DialMacHosts 连接 mac 方言服务器,遍历多服务器直至成功。
func DialMacHosts(hosts []string, op ...client.Option) (*Client, error) {
	return dialMacWith(NewMacRangeDial(hosts), op...)
}

// dialMacWith 建立 mac 方言连接: 握手用 3 条 setup 命令, 心跳复用第一条 setup。
func dialMacWith(dial ios.DialFunc, op ...client.Option) (cli *Client, err error) {
	cli = &Client{
		Wait: wait.New(time.Second * 10),
		m:    maps.NewSafe(),
	}
	cli.Client, err = client.Dial(dial, func(c *client.Client) {
		c.Logger.Debug(true)                           //关闭日志打印
		c.Logger.SetLevel(LevelInfo)                   //设置日志级别
		c.Logger.WithHEX()                             //以HEX显示
		c.SetOption(op...)                             //自定义选项
		c.Event.OnReadFrom = protocol.ReadFrom         //分包(与标准行情相同的响应信封)
		c.Event.OnDealMessage = cli.handlerDealMessage //解析数据并处理
		c.Event.OnConnected = func(c *client.Client) error {
			// 握手: 3条 setup 命令(响应由分发器按 TypeConnect/TypeLogin2 忽略)
			for _, cmd := range protocol.MacSetupCommands {
				if _, err := c.Write(cmd); err != nil {
					c.Close()
					return err
				}
			}
			// 心跳: 无数据超时60秒,30秒发送一次(响应无等待者,被分发器丢弃)
			c.GoTimerWriter(30*time.Second, func(w ios.MoreWriter) error {
				_, err := w.Write(protocol.MacSetupCommands[0])
				return err
			})
			return nil
		}
	})
	if err != nil {
		return nil, err
	}
	go cli.Client.Run()
	return cli, nil
}

// ---- mac 方言 API ----

// GetMacTrade 最近交易时段逐笔成交(秒级时间+成交笔数,含盘后固定价格成交)。
// start 从最新一端往回偏移(start=0 返回最新的 count 笔),单次 count 建议 ≤1000。
// 注意: 记录时间日期部分为客户端当日(与标准 GetTrade 一致),非交易日拉取的是最近交易日数据。
func (this *Client) GetMacTrade(code string, start uint32, count uint16) (*protocol.MacTradeResp, error) {
	return this.getMacTrade(code, 0, start, count)
}

// GetMacHistoryTrade 指定日期逐笔成交(date=YYYYMMDD)。
func (this *Client) GetMacHistoryTrade(date, code string, start uint32, count uint16) (*protocol.MacTradeResp, error) {
	ymd, err := parseMacDate(date)
	if err != nil {
		return nil, err
	}
	return this.getMacTrade(code, ymd, start, count)
}

// GetMacTradeAll 最近交易时段全部逐笔成交(自动并发分页,拼接为时间正序)。
// 建议盘后调用: 盘中各页间新增成交会丢(同标准 GetTradeAll)。
func (this *Client) GetMacTradeAll(code string) (*protocol.MacTradeResp, error) {
	return this.getMacTradeAll(code, 0)
}

// GetMacHistoryTradeAll 指定日期全部逐笔成交(date=YYYYMMDD,自动并发分页,拼接为时间正序)。
func (this *Client) GetMacHistoryTradeAll(date, code string) (*protocol.MacTradeResp, error) {
	ymd, err := parseMacDate(date)
	if err != nil {
		return nil, err
	}
	return this.getMacTradeAll(code, ymd)
}

func (this *Client) getMacTrade(code string, ymd, start uint32, count uint16) (*protocol.MacTradeResp, error) {
	f, err := protocol.MMacTrade.Frame(code, ymd, start, count)
	if err != nil {
		return nil, err
	}
	date := time.Now().Format("20060102")
	if ymd > 0 {
		date = fmt.Sprintf("%08d", ymd)
	}
	r, err := this.SendFrame(f, protocol.MacTradeCache{Date: date, Code: code})
	if err != nil {
		return nil, err
	}
	return r.(*protocol.MacTradeResp), nil
}

// macMaxConcurrency 并发分页的并发数上限(公共服务器,避免过大压力)
const macMaxConcurrency = 5

// getMacTradeAll 全量分页: 首页探测 total,剩余页并发拉取,前插拼接为时间正序。
// 各页几乎同时发出,"最新一端"参照点一致,盘中取数时页间错位风险反而小于串行分页。
func (this *Client) getMacTradeAll(code string, ymd uint32) (*protocol.MacTradeResp, error) {
	const size = uint16(1000)
	first, err := this.getMacTrade(code, ymd, 0, size)
	if err != nil {
		return nil, err
	}
	resp := &protocol.MacTradeResp{Count: first.Count, Total: first.Total, List: first.List}
	remain := int64(first.Total) - int64(first.Count)
	if remain <= 0 {
		return resp, nil
	}

	//剩余页并发拉取(带限流)
	rest := (remain + int64(size) - 1) / int64(size)
	type pageResult struct {
		page *protocol.MacTradeResp
		err  error
	}
	results := make([]pageResult, rest)
	sem := make(chan struct{}, macMaxConcurrency)
	var wg sync.WaitGroup
	for i := int64(0); i < rest; i++ {
		wg.Add(1)
		go func(i int64) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i].page, results[i].err = this.getMacTrade(code, ymd, uint32(i+1)*uint32(size), size)
		}(i)
	}
	wg.Wait()
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
	}

	//start=0 为最新一端,页内升序页间倒序,依次前插拼接为时间正序
	//(i=0 即 start=size 的页比首页老一档,逐次把更老的页插到最前)
	for i := int64(0); i < rest; i++ {
		resp.List = append(results[i].page.List, resp.List...)
		resp.Count += results[i].page.Count
	}
	return resp, nil
}

// GetMacQuote mac 批量自定义字段报价(0x122B, 实时)。
// 含标准 GetQuote 没有的字段: 今日主力净流入/内外盘/均价/涨速/量比/涨跌停价/PE/盘后量等。
// codes 需可解析(带前缀或 6 位数字); 单次请求服务器上限 80 只, 超出自动分批。
func (this *Client) GetMacQuote(codes ...string) ([]*protocol.MacQuote, error) {
	const batch = 80
	out := make([]*protocol.MacQuote, 0, len(codes))
	for i := 0; i < len(codes); i += batch {
		end := i + batch
		if end > len(codes) {
			end = len(codes)
		}
		f, err := protocol.MMacQuote.Frame(codes[i:end])
		if err != nil {
			return nil, err
		}
		r, err := this.SendFrame(f)
		if err != nil {
			return nil, err
		}
		out = append(out, r.([]*protocol.MacQuote)...)
	}
	return out, nil
}

// GetMacCapitalFlow 个股资金流向(实时, 通达信口径)。
func (this *Client) GetMacCapitalFlow(code string) (*protocol.MacCapitalFlow, error) {
	f, err := protocol.MMacCapitalFlow.Frame(code)
	if err != nil {
		return nil, err
	}
	r, err := this.SendFrame(f, protocol.MacCapitalFlowCache{})
	if err != nil {
		return nil, err
	}
	return r.(*protocol.MacCapitalFlow), nil
}

// GetMacBelongBoards 个股所属板块。
func (this *Client) GetMacBelongBoards(code string) ([]*protocol.MacBelongBoard, error) {
	f, err := protocol.MMacBelongBoard.Frame(code)
	if err != nil {
		return nil, err
	}
	r, err := this.SendFrame(f, protocol.MacBelongBoardCache{})
	if err != nil {
		return nil, err
	}
	return r.([]*protocol.MacBelongBoard), nil
}

// GetMacBoardMembers 板块成分报价(按涨幅降序, 实时), 返回字段见 macBoardMemberFields。
// board 板块代码(如 881160); start 分页偏移; count 单页数量(≤80, 超出会被服务端截断)。
func (this *Client) GetMacBoardMembers(board string, start, count uint16) ([]*protocol.MacQuote, error) {
	f, err := protocol.MMacBoardMembers.Frame(board, protocol.MacSortByChangePct, uint32(start), count, protocol.MacSortDesc)
	if err != nil {
		return nil, err
	}
	r, err := this.SendFrame(f)
	if err != nil {
		return nil, err
	}
	return r.([]*protocol.MacQuote), nil
}

// GetMacServerSession 服务器交易时段与交易日历。
func (this *Client) GetMacServerSession() (*protocol.MacServerSession, error) {
	r, err := this.SendFrame(protocol.MMacServerSession.Frame())
	if err != nil {
		return nil, err
	}
	return r.(*protocol.MacServerSession), nil
}

// GetMacKlineCount K线数据总量(可兼作轻量探活)。
func (this *Client) GetMacKlineCount() (*protocol.MacKlineCount, error) {
	r, err := this.SendFrame(protocol.MMacKlineCount.Frame())
	if err != nil {
		return nil, err
	}
	return r.(*protocol.MacKlineCount), nil
}

// GetMacFileInfo 远程文件元信息(大小/哈希)。
func (this *Client) GetMacFileInfo(name string) (*protocol.MacFileMeta, error) {
	r, err := this.SendFrame(protocol.MMacFileMeta.Frame(name, 0))
	if err != nil {
		return nil, err
	}
	return r.(*protocol.MacFileMeta), nil
}

// GetMacFileChunk 远程文件分段下载(单段建议 30000 字节)。
func (this *Client) GetMacFileChunk(name string, index, offset, size uint32) ([]byte, error) {
	r, err := this.SendFrame(protocol.MMacFileData.Frame(name, index, offset, size))
	if err != nil {
		return nil, err
	}
	return r.([]byte), nil
}

// macFileChunkSize 文件下载单段大小
const macFileChunkSize = 30000

// GetMacFile 远程文件完整下载(按元信息自动分段拼接, 上限 64MB)。
func (this *Client) GetMacFile(name string) ([]byte, error) {
	meta, err := this.GetMacFileInfo(name)
	if err != nil {
		return nil, err
	}
	if meta.Size <= 0 {
		return nil, fmt.Errorf("文件不存在或为空: %s (size=%d flag=%d hash=%s)", name, meta.Size, meta.Flag, meta.Hash)
	}
	if meta.Size > 64<<20 {
		return nil, fmt.Errorf("文件过大(%d 字节), 超过 64MB 上限", meta.Size)
	}
	out := make([]byte, 0, meta.Size)
	for index, offset := uint32(1), uint32(0); offset < uint32(meta.Size); index, offset = index+1, offset+macFileChunkSize {
		chunk, err := this.GetMacFileChunk(name, index, offset, macFileChunkSize)
		if err != nil {
			return nil, err
		}
		out = append(out, chunk...)
		if len(chunk) < macFileChunkSize {
			break
		}
	}
	return out, nil
}

// parseMacDate 解析 YYYYMMDD 日期字符串为 uint32
func parseMacDate(s string) (uint32, error) {
	t, err := time.Parse("20060102", s)
	if err != nil {
		return 0, fmt.Errorf("日期格式应为 YYYYMMDD: %w", err)
	}
	return uint32(t.Year()*10000 + int(t.Month())*100 + t.Day()), nil
}
