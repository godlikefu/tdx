package protocol

import (
	"errors"
	"strconv"
)

/*
mac 板块成分报价(0x122C, 实时)。

请求 = 板块代码(I, 如 881160) + 9B 保留 + 排序字段(H) + start(I) + page(H)
     + 排序方向(B) + 保留(B) + 16B 字段位图 + 4B 控制区(盘口/排除位/日内/扩展=1)。
响应格式与 0x122B 相同(20B 位图回显 + total + rowCount + 行)。
逆向来源: pytdx / easy_tdx mac/commands/board_members_quotes.py。
*/

var (
	MMacBoardMembers = macBoardMembers{}
)

// mac 板块排序字段(SortType)
const (
	MacSortByPrice     = 0x06 //最新价
	MacSortByVolume    = 0x09 //成交量
	MacSortByAmount    = 0x0A //成交额
	MacSortByChangePct = 0x0E //涨幅%
	MacSortByTurnover  = 0x1B //换手(注意: easy_tdx SortType 无此项, 0x1B 为换手字段位, 有效性未验证)
	MacSortByMainNet   = 0x38 //主力净流入(有效性未验证)
)

// mac 排序方向(SortOrder)
const (
	MacSortNone = 0x00
	MacSortDesc = 0x01
	MacSortAsc  = 0x02
)

// 板块成分请求的字段集(按 bit 升序; 结果映射到 MacQuote 对应字段)
var macBoardMemberFields = []macQuoteFieldDef{
	{mqPreClose, mqKindPrice},
	{mqClose, mqKindPrice},
	{mqVolume, mqKindUint},
	{mqAmount, mqKindFloat},
	{mqTurnover, mqKindFloat},
	{mqSpeedPct, mqKindFloat},
	{mqMainNetAmount, mqKindFloat},
}

// macBoardMembersBitmap 请求位图(16B 字段位图 + 4B 控制区: 0/0/0/1 扩展模式)
var macBoardMembersBitmap = func() []byte {
	b := make([]byte, 20)
	for _, f := range macBoardMemberFields {
		b[f.bit/8] |= 1 << (f.bit % 8)
	}
	//控制区 byte3=1: CTRL_EXTENDED 扩展模式(含北交所等)
	b[19] = 1
	return b
}()

type macBoardMembers struct{}

// convertMacBoardCode 板块显示代码 → 服务器协议代码
// (规则来源 opentdx exchange_board_code: 880216→20816, 399372→30372, 899050→32050, 000686→31686)
func convertMacBoardCode(board string) (uint32, error) {
	s := board
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, errors.New("板块代码应为数字, 如 881160")
	}
	switch {
	case len(s) == 6 && s[:2] == "88":
		return uint32(n - 880000 + 20000), nil
	case len(s) == 6 && s[:3] == "399":
		return uint32(n - 399000 + 30000), nil
	case len(s) == 6 && s[:3] == "899":
		return uint32(n - 899000 + 32000), nil
	case len(s) == 6 && s[:3] == "000":
		return uint32(31000 + n), nil
	default:
		return uint32(n), nil
	}
}

// Frame 构造板块成分报价请求。
// board 板块显示代码(如 881160, 内部转换为协议代码); start 从 0 起按涨幅排序的偏移; count 单页数量(≤80)。
func (macBoardMembers) Frame(board string, sortType uint8, start uint32, count uint16, sortOrder uint8) (*Frame, error) {
	code, err := convertMacBoardCode(board)
	if err != nil {
		return nil, err
	}
	body := make([]byte, 23+20) //固定部分23B(4+9+2+4+2+1+1) + 位图20B
	body[0] = byte(code)
	body[1] = byte(code >> 8)
	body[2] = byte(code >> 16)
	body[3] = byte(code >> 24)
	//9B 保留
	body[13] = byte(sortType)
	body[15] = byte(start)
	body[16] = byte(start >> 8)
	body[17] = byte(start >> 16)
	body[18] = byte(start >> 24)
	body[19] = byte(count)
	body[20] = byte(count >> 8)
	body[21] = sortOrder
	//body[22] 保留 0
	copy(body[23:39], macBoardMembersBitmap[:16])
	copy(body[39:43], macBoardMembersBitmap[16:20])
	return &Frame{
		Prefix:  PrefixMac,
		Control: Control01,
		Type:    TypeMacBoardMembers,
		Data:    body,
	}, nil
}

// Decode 解析板块成分报价响应(行格式与 0x122B 相同)。
func (macBoardMembers) Decode(bs []byte) ([]*MacQuote, error) {
	return decodeMacQuoteRows(bs, macBoardMemberFields)
}
