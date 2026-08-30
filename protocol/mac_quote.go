package protocol

import (
	"errors"
	"fmt"
	"math"
)

/*
mac 批量自定义字段报价(0x122B, 实时)。

按字段位图请求多只证券的行情, 字段表远超标准 GetQuote(0x053E):
主力净流入/内外盘/均价/涨速/量比/涨跌停价/PE/盘后量等。
逆向来源: pytdx / easy_tdx codec/bitmap.py + mac/commands/symbol_quotes.py。

请求体: 20B 位图(位 i = 字段 0x%02X; 位 128-159 与控制区重叠, 实测服务器支持 0x80+ 盘口位)
       + count(H) + count × (market(H) + code(22B GBK 补零))
响应体: 20B 位图回显 + total(I) + rowCount(H)
       + rowCount × (68B 头: market(H) + code(22B) + name(44B GBK)
       + 字段数 × 4B 值, 按 bit 升序排列, 单次建议 ≤80 只)
*/

var (
	MMacQuote = macQuote{}
)

// mac 字段位定义(仅列出本库请求的字段; 位图 20B=160 位, 0x80+ 为五档盘口)
const (
	mqPreClose       = 0x00 //昨收
	mqOpen           = 0x01 //开盘价
	mqHigh           = 0x02 //最高价
	mqLow            = 0x03 //最低价
	mqClose          = 0x04 //最新价
	mqVolume         = 0x05 //成交量(<I)
	mqVolRatio       = 0x06 //量比
	mqAmount         = 0x07 //成交额(元)
	mqInsideVolume   = 0x08 //内盘(<I)
	mqOutsideVolume  = 0x09 //外盘(<I)
	mqTotalShares    = 0x0A //总股数(万)
	mqFloatShares    = 0x0B //流通股(万)
	mqPE             = 0x10 //市盈率(动)
	mqLastVolume     = 0x1A //现量(<I)
	mqTurnover       = 0x1B //换手%
	mqBuyPriceLimit  = 0x20 //涨停价
	mqSellPriceLimit = 0x21 //跌停价
	mqSpeedPct       = 0x25 //涨速%
	mqAvgPrice       = 0x26 //均价
	mqAfterHoursVol  = 0x2E //盘后量(实为 float32)
	mqPeTTM          = 0x30 //市盈率TTM
	mqPeStatic       = 0x31 //市盈率(静)
	mqMainNetAmount  = 0x38 //今日主力净流入(元)
	mqVolSpeedPct    = 0x68 //量涨速%
	mqMainNetRatio   = 0x6C //主力净比%
	mqBidPrice1      = 0x11 //买一价
	mqAskPrice1      = 0x12 //卖一价
	mqBidVol1        = 0x18 //买一量(<I)
	mqAskVol1        = 0x19 //卖一量(<I)
	mqEntrustRatio   = 0x39 //委比%
	mqBidPrice2      = 0x48 //买二价
	mqAskPrice2      = 0x49 //卖二价
	mqBidVol2        = 0x5D //买二量(<I)
	mqAskVol2        = 0x5E //卖二量(<I)
	mqBidPrice3      = 0x80 //买三价
	mqBidPrice4      = 0x81 //买四价
	mqBidPrice5      = 0x82 //买五价
	mqAskPrice3      = 0x83 //卖三价
	mqAskPrice4      = 0x84 //卖四价
	mqAskPrice5      = 0x85 //卖五价
	mqBidVol3        = 0x86 //买三量(<I)
	mqBidVol4        = 0x87 //买四量(<I)
	mqBidVol5        = 0x88 //买五量(<I)
	mqAskVol3        = 0x89 //卖三量(<I)
	mqAskVol4        = 0x8A //卖四量(<I)
	mqAskVol5        = 0x8B //卖五量(<I)
)

// mac 字段类型
const (
	mqKindPrice uint8 = iota //float32 元 → Price 厘
	mqKindUint               //uint32
	mqKindFloat              //float32
)

// macQuoteFieldDef 字段定义(切片必须按 bit 升序, 与响应字段排列顺序一致)
type macQuoteFieldDef struct {
	bit  uint16
	kind uint8
}

var macQuoteFields = []macQuoteFieldDef{
	{mqPreClose, mqKindPrice},
	{mqOpen, mqKindPrice},
	{mqHigh, mqKindPrice},
	{mqLow, mqKindPrice},
	{mqClose, mqKindPrice},
	{mqVolume, mqKindUint},
	{mqVolRatio, mqKindFloat},
	{mqAmount, mqKindFloat},
	{mqInsideVolume, mqKindUint},
	{mqOutsideVolume, mqKindUint},
	{mqTotalShares, mqKindFloat},
	{mqFloatShares, mqKindFloat},
	{mqPE, mqKindFloat},
	{mqBidPrice1, mqKindPrice},
	{mqAskPrice1, mqKindPrice},
	{mqBidVol1, mqKindUint},
	{mqAskVol1, mqKindUint},
	{mqLastVolume, mqKindUint},
	{mqTurnover, mqKindFloat},
	{mqBuyPriceLimit, mqKindPrice},
	{mqSellPriceLimit, mqKindPrice},
	{mqSpeedPct, mqKindFloat},
	{mqAvgPrice, mqKindPrice},
	{mqAfterHoursVol, mqKindFloat},
	{mqPeTTM, mqKindFloat},
	{mqPeStatic, mqKindFloat},
	{mqMainNetAmount, mqKindFloat},
	{mqEntrustRatio, mqKindFloat},
	{mqBidPrice2, mqKindPrice},
	{mqAskPrice2, mqKindPrice},
	{mqBidVol2, mqKindUint},
	{mqAskVol2, mqKindUint},
	{mqVolSpeedPct, mqKindFloat},
	{mqMainNetRatio, mqKindFloat},
	{mqBidPrice3, mqKindPrice},
	{mqBidPrice4, mqKindPrice},
	{mqBidPrice5, mqKindPrice},
	{mqAskPrice3, mqKindPrice},
	{mqAskPrice4, mqKindPrice},
	{mqAskPrice5, mqKindPrice},
	{mqBidVol3, mqKindUint},
	{mqBidVol4, mqKindUint},
	{mqBidVol5, mqKindUint},
	{mqAskVol3, mqKindUint},
	{mqAskVol4, mqKindUint},
	{mqAskVol5, mqKindUint},
}

// macQuoteBitmap 请求位图(16B 字段位图 + 4B 控制区0)
var macQuoteBitmap = func() []byte {
	b := make([]byte, 20)
	for _, f := range macQuoteFields {
		b[f.bit/8] |= 1 << (f.bit % 8)
	}
	return b
}()

type macQuote struct{}

// Frame 构造 mac 批量报价请求。codes 需可解析(带前缀或 6 位数字), 单次建议 ≤80 只。
func (macQuote) Frame(codes []string) (*Frame, error) {
	if len(codes) == 0 {
		return nil, errors.New("代码不能为空")
	}
	body := make([]byte, 0, 20+2+24*len(codes))
	body = append(body, macQuoteBitmap...)
	body = append(body, byte(len(codes)), byte(len(codes)>>8))
	for _, code := range codes {
		exchange, number, err := DecodeCode(code)
		if err != nil {
			return nil, err
		}
		body = append(body, byte(exchange.Uint8()), 0x00)
		b := make([]byte, 22)
		copy(b, number)
		body = append(body, b...)
	}
	return &Frame{
		Prefix:  PrefixMac,
		Control: Control01,
		Type:    TypeMacQuote,
		Data:    body,
	}, nil
}

// MacQuote mac 批量报价记录(实时)。
type MacQuote struct {
	Market uint8  //市场: 0=SZ 1=SH
	Code   string //代码
	Name   string //名称(GBK 已转 UTF-8)

	PreClose       Price   //昨收(厘)
	Open           Price   //开盘价(厘)
	High           Price   //最高价(厘)
	Low            Price   //最低价(厘)
	Price          Price   //最新价(厘)
	AvgPrice       Price   //均价(厘)
	BuyPriceLimit  Price   //涨停价(厘)
	SellPriceLimit Price   //跌停价(厘)
	Volume         int64   //成交量
	VolRatio       float64 //量比
	Amount         float64 //成交额(元)
	InsideVolume   int64   //内盘
	OutsideVolume  int64   //外盘
	TotalShares    float64 //总股数(万)
	FloatShares    float64 //流通股(万)
	PE             float64 //市盈率(动)
	LastVolume     int64   //现量
	Turnover       float64 //换手%
	SpeedPct       float64 //涨速%
	AfterHoursVol  float64 //盘后量(手; easy_tdx 类型表标 <i 有误, 实测 1192547328=float32(36912.0))
	PeTTM          float64 //市盈率TTM
	PeStatic       float64 //市盈率(静)
	MainNetAmount  float64 //今日主力净流入(元, 厂商口径, 与腾讯/东财同源)
	VolSpeedPct    float64 //量涨速%
	MainNetRatio   float64 //主力净比%
	EntrustRatio   float64 //委比%

	BidPrice1 Price //买一价(厘)
	BidPrice2 Price //买二价
	BidPrice3 Price //买三价
	BidPrice4 Price //买四价
	BidPrice5 Price //买五价
	AskPrice1 Price //卖一价(厘)
	AskPrice2 Price //卖二价
	AskPrice3 Price //卖三价
	AskPrice4 Price //卖四价
	AskPrice5 Price //卖五价
	BidVol1   int64 //买一量(手)
	BidVol2   int64 //买二量
	BidVol3   int64 //买三量
	BidVol4   int64 //买四量
	BidVol5   int64 //买五量
	AskVol1   int64 //卖一量(手)
	AskVol2   int64 //卖二量
	AskVol3   int64 //卖三量
	AskVol4   int64 //卖四量
	AskVol5   int64 //卖五量
}

func (this *MacQuote) String() string {
	return fmt.Sprintf("%s %s \t%-7.3f \t主力净流入 %.0f(元) \t量比 %.2f \t换手 %.2f%%",
		this.Code, this.Name,
		this.Price.Float64(), this.MainNetAmount, this.VolRatio, this.Turnover)
}

// Decode 解析 mac 批量报价响应。
func (macQuote) Decode(bs []byte) ([]*MacQuote, error) {
	return decodeMacQuoteRows(bs, macQuoteFields)
}

// decodeMacQuoteRows 解析 0x122B/0x122C 共用的行格式响应:
// 20B 位图回显 + total(I) + rowCount(H) + rowCount×(68B 头 + 字段数×4B)。
// defs 为请求的字段定义(按 bit 升序), 与响应位图取交集决定实际字段。
func decodeMacQuoteRows(bs []byte, defs []macQuoteFieldDef) ([]*MacQuote, error) {
	if len(bs) < 26 {
		return nil, errors.New("数据长度不足")
	}
	rowCount := int(Uint16(bs[24:26]))
	//位图回显 20B(位 128-159 与控制区重叠)。若服务器对高位字段"原样回显但不打包值",
	//行长度会与数据不符, 此时回退按低 128 位解析
	active := macQuoteActiveFields(bs[:20], defs)
	rowLen := 68 + 4*len(active)
	if rowCount > 0 && 26+rowCount*rowLen > len(bs) {
		if fb := macQuoteActiveFields(bs[:16], defs); len(fb) != len(active) {
			active = fb
			rowLen = 68 + 4*len(active)
		}
	}

	out := make([]*MacQuote, 0, rowCount)
	pos := 26
	for i := 0; i < rowCount && pos+rowLen <= len(bs); i++ {
		row := bs[pos : pos+rowLen]
		pos += rowLen
		q := &MacQuote{
			Market: uint8(Uint16(row[0:2])),
			Code:   string(UTF8ToGBK(row[2:24])),
			Name:   string(UTF8ToGBK(row[24:68])),
		}
		off := 68
		for _, f := range active {
			v := row[off : off+4]
			off += 4
			switch f.bit {
			case mqPreClose:
				q.PreClose = f.price(v)
			case mqOpen:
				q.Open = f.price(v)
			case mqHigh:
				q.High = f.price(v)
			case mqLow:
				q.Low = f.price(v)
			case mqClose:
				q.Price = f.price(v)
			case mqAvgPrice:
				q.AvgPrice = f.price(v)
			case mqBuyPriceLimit:
				q.BuyPriceLimit = f.price(v)
			case mqSellPriceLimit:
				q.SellPriceLimit = f.price(v)
			case mqVolume:
				q.Volume = int64(Uint32(v))
			case mqLastVolume:
				q.LastVolume = int64(Uint32(v))
			case mqInsideVolume:
				q.InsideVolume = int64(Uint32(v))
			case mqOutsideVolume:
				q.OutsideVolume = int64(Uint32(v))
			case mqAfterHoursVol:
				q.AfterHoursVol = f.float(v)
			case mqVolRatio:
				q.VolRatio = f.float(v)
			case mqAmount:
				q.Amount = f.float(v)
			case mqTotalShares:
				q.TotalShares = f.float(v)
			case mqFloatShares:
				q.FloatShares = f.float(v)
			case mqPE:
				q.PE = f.float(v)
			case mqTurnover:
				q.Turnover = f.float(v)
			case mqSpeedPct:
				q.SpeedPct = f.float(v)
			case mqPeTTM:
				q.PeTTM = f.float(v)
			case mqPeStatic:
				q.PeStatic = f.float(v)
			case mqMainNetAmount:
				q.MainNetAmount = f.float(v)
			case mqVolSpeedPct:
				q.VolSpeedPct = f.float(v)
			case mqMainNetRatio:
				q.MainNetRatio = f.float(v)
			case mqEntrustRatio:
				q.EntrustRatio = f.float(v)
			case mqBidPrice1:
				q.BidPrice1 = f.price(v)
			case mqBidPrice2:
				q.BidPrice2 = f.price(v)
			case mqBidPrice3:
				q.BidPrice3 = f.price(v)
			case mqBidPrice4:
				q.BidPrice4 = f.price(v)
			case mqBidPrice5:
				q.BidPrice5 = f.price(v)
			case mqAskPrice1:
				q.AskPrice1 = f.price(v)
			case mqAskPrice2:
				q.AskPrice2 = f.price(v)
			case mqAskPrice3:
				q.AskPrice3 = f.price(v)
			case mqAskPrice4:
				q.AskPrice4 = f.price(v)
			case mqAskPrice5:
				q.AskPrice5 = f.price(v)
			case mqBidVol1:
				q.BidVol1 = int64(Uint32(v))
			case mqBidVol2:
				q.BidVol2 = int64(Uint32(v))
			case mqBidVol3:
				q.BidVol3 = int64(Uint32(v))
			case mqBidVol4:
				q.BidVol4 = int64(Uint32(v))
			case mqBidVol5:
				q.BidVol5 = int64(Uint32(v))
			case mqAskVol1:
				q.AskVol1 = int64(Uint32(v))
			case mqAskVol2:
				q.AskVol2 = int64(Uint32(v))
			case mqAskVol3:
				q.AskVol3 = int64(Uint32(v))
			case mqAskVol4:
				q.AskVol4 = int64(Uint32(v))
			case mqAskVol5:
				q.AskVol5 = int64(Uint32(v))
			}
		}
		out = append(out, q)
	}
	return out, nil
}

// price float32 元 → Price 厘
func (f macQuoteFieldDef) price(v []byte) Price {
	return Price(math.Round(float64(Float32(v)) * 1000))
}

func (f macQuoteFieldDef) float(v []byte) float64 {
	return float64(Float32(v))
}

// macQuoteActiveFields 从响应位图取活跃字段(按 bit 升序)
func macQuoteActiveFields(bitmap []byte, defs []macQuoteFieldDef) []macQuoteFieldDef {
	out := make([]macQuoteFieldDef, 0, len(defs))
	for _, f := range defs {
		if bitmap[f.bit/8]&(1<<(f.bit%8)) != 0 {
			out = append(out, f)
		}
	}
	return out
}
