package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"
)

/*
mac 方言(通达信 MAC 版客户端协议)。

传输信封与标准 7709 完全相同(16 字节响应头 b1cb7400 + zlib 可选压缩 + 心跳),
仅内层命令帧不同,且与标准帧结构同构:

	标准: [0]=0x0C(Prefix) [1:5]=MsgID(4B) [5]=Control [6:10]=长度×2 [10:12]=Type [12:]=Data
	mac:  [0]=0x1C(Prefix) [1:5]=MsgID(4B) [5]=0x01    [6:10]=长度×2 [10:12]=命令号  [12:]=Data

即 Frame{Prefix: PrefixMac, Control: Control01, Type: mac命令号}。请求 [1:5] 可携带
MsgID,响应会原样回显(2026-08-29 实测),因此 tdx 的 MsgID/Wait 机制无需改动。

mac 服务器与标准行情服务器不重叠,端口同为 7709,连接后需按序发送 MacSetupCommands
三条握手(每条响应需被分发/丢弃)。逆向来源: pytdx / easy_tdx(easy_tdx 侧已与东方财富
逐笔对账);2026-08-29 本实现与 easy_tdx mac 客户端逐行对账 100% 一致(sh601872,4597 笔)。
*/

var (
	MMacTrade = macTrade{}
)

// MacSetupCommands mac 服务器握手命令(3 条,来源 pytdx setup_commands,已在真实 mac 服务器验证)。
// 响应 Type: 第一、二条 0x000D(同 TypeConnect),第三条 0x0FDB(TypeLogin2)。
// 心跳可周期性发送第一条。
var MacSetupCommands = [][]byte{
	{0x0c, 0x02, 0x18, 0x93, 0x00, 0x01, 0x03, 0x00, 0x03, 0x00, 0x0d, 0x00, 0x01},
	{0x0c, 0x02, 0x18, 0x94, 0x00, 0x01, 0x03, 0x00, 0x03, 0x00, 0x0d, 0x00, 0x02},
	{0x0c, 0x03, 0x18, 0x99, 0x00, 0x01, 0x20, 0x00, 0x20, 0x00, 0xdb, 0x0f, 0xd5, 0xd0, 0xc9, 0xcc, 0xd6, 0xa4, 0xa8, 0xaf, 0x00, 0x00, 0x00, 0x8f, 0xc2, 0x25, 0x40, 0x13, 0x00, 0x00, 0xd5, 0x00, 0xc9, 0xcc, 0xbd, 0xf0, 0xd7, 0xea, 0x00, 0x00, 0x00, 0x02},
}

type macTrade struct{}

/*
Frame 构造 mac 逐笔成交请求(命令号 0x122F)。

请求体: market(H) + code(22B 补零) + ymd(I) + start(I) + count(H) + 10B 保留零。
ymd=0 表示最近交易日;start 从最新一端往回偏移(start=0 返回最新的 count 笔);
count 单次建议 ≤1000。
*/
func (macTrade) Frame(code string, ymd uint32, start uint32, count uint16) (*Frame, error) {
	exchange, number, err := DecodeCode(code)
	if err != nil {
		return nil, err
	}
	body := make([]byte, 2+22+4+4+2+10)
	binary.LittleEndian.PutUint16(body[0:2], uint16(exchange.Uint8()))
	copy(body[2:24], number)
	binary.LittleEndian.PutUint32(body[24:28], ymd)
	binary.LittleEndian.PutUint32(body[28:32], start)
	binary.LittleEndian.PutUint16(body[32:34], count)
	// 尾部 10 字节保持 0
	return &Frame{
		Prefix:  PrefixMac,
		Control: Control01, //mac 帧 version 位,恰与标准 Control01 同值
		Type:    TypeMacTransaction,
		Data:    body,
	}, nil
}

// MacTrade mac 逐笔成交(秒级时间,含盘后固定价格成交)。
type MacTrade struct {
	Time       time.Time //成交时间,秒级(日期部分由请求日期/客户端当日补齐)
	Price      Price     //价格(厘)
	Volume     int64     //成交量,手
	TradeCount int64     //成交笔数(撮合笔数,与东方财富 details 接口第 4 列一致)
	Status     int       //成交性质: 0=买入 1=卖出 2=中性/集合竞价 5=盘后固定价格成交
}

func (this *MacTrade) String() string {
	return fmt.Sprintf("%s \t%-8.3f \t%-6d(手) \t%-4d(笔) \t%s",
		this.Time.Format("2006-01-02 15:04:05"),
		this.Price.Float64(),
		this.Volume, this.TradeCount, this.StatusString())
}

// StatusString 成交性质描述。
func (this *MacTrade) StatusString() string {
	switch this.Status {
	case 0:
		return "买入"
	case 1:
		return "卖出"
	case 5:
		return "盘后"
	default:
		return ""
	}
}

// MacTradeResp mac 逐笔成交响应。
type MacTradeResp struct {
	Count uint16 //本页条数
	Total uint32 //最近端往回的全部条数(服务器返回)
	List  []*MacTrade
}

// MacTradeCache 解析缓存,用于补齐记录时间的日期部分。
type MacTradeCache struct {
	Date string //日期 YYYYMMDD
	Code string //代码
}

/*
Decode 解析 mac 逐笔成交响应。

响应体: 39 字节头(market 2 + code 22 + ymd 4 + flag 1 + count 2 + start 4 + total 4)
+ count × 18 字节定长记录: 当日秒(u32) + 价格 f32(元) + 量手(u32) + 成交笔数(u32) + 性质(u16)。

记录顺序: 页内时间升序;页间由新到旧(start=0 为最新一端),自动分页时需前插拼接。
*/
func (macTrade) Decode(bs []byte, cache MacTradeCache) (*MacTradeResp, error) {
	if len(bs) < 39 {
		return nil, errors.New("数据长度不足")
	}
	resp := &MacTradeResp{
		Count: Uint16(bs[29:31]),
		Total: Uint32(bs[35:39]),
	}
	if len(bs) < 39+int(resp.Count)*18 {
		return nil, fmt.Errorf("数据长度不足: 预期%d,得到%d", 39+int(resp.Count)*18, len(bs))
	}

	date, err := time.Parse("20060102", cache.Date)
	if err != nil {
		return nil, err
	}

	resp.List = make([]*MacTrade, 0, resp.Count)
	for i := 0; i < int(resp.Count); i++ {
		off := 39 + i*18
		sec := Uint32(bs[off : off+4])
		resp.List = append(resp.List, &MacTrade{
			Time: time.Date(date.Year(), date.Month(), date.Day(),
				int(sec/3600), int(sec%3600/60), int(sec%60), 0, time.Local),
			Price:      Price(math.Round(float64(Float32(bs[off+4:off+8])) * 1000)), //元→厘
			Volume:     int64(Uint32(bs[off+8 : off+12])),
			TradeCount: int64(Uint32(bs[off+12 : off+16])),
			Status:     int(Uint16(bs[off+16 : off+18])),
		})
	}
	return resp, nil
}
