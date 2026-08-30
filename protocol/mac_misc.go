package protocol

import (
	"errors"
	"fmt"
	"time"
)

/*
mac 杂项命令: 服务器交易时段(0x120F) / K线总量(0x124A) / 远程文件(0x1215/0x1217)。
逆向来源: pytdx / easy_tdx mac/commands/server_info.py + kline_offset.py + file_query.py。
*/

var (
	MMacServerSession = macServerSession{}
	MMacKlineCount    = macKlineCount{}
	MMacFileMeta      = macFileMeta{}
	MMacFileData      = macFileData{}
)

// MacSession 交易时段(分钟数从 0 点起)
type MacSession struct {
	Open  int //开市分钟数
	Close int //收市分钟数
}

func (this MacSession) String() string {
	return fmt.Sprintf("%02d:%02d-%02d:%02d", this.Open/60, this.Open%60, this.Close/60, this.Close%60)
}

// MacServerSession 服务器交易时段信息(含交易日历)。
type MacServerSession struct {
	Today          time.Time     //当前日期
	LastTradingDay time.Time     //最近交易日
	Sessions1      [4]MacSession //交易时段组1(A股: 09:30-11:30 / 13:00-15:00)
	Sessions2      [4]MacSession //交易时段组2
	MarketParam1   uint32        //市场参数(含义未知)
	MarketParam2   uint32        //市场参数(含义未知)
}

type macServerSession struct{}

// Frame 构造交易时段查询请求(固定 68B)。
func (macServerSession) Frame() *Frame {
	body := make([]byte, 68)
	body[0], body[1], body[2], body[3] = 0x04, 0x00, 0x2d, 0x31
	body[12], body[13], body[14], body[15] = 0x00, 0x27, 0x06, 0x0e
	return &Frame{
		Prefix:  PrefixMac,
		Control: Control01,
		Type:    TypeMacServerSession,
		Data:    body,
	}
}

func macDate(v uint32) time.Time {
	return time.Date(int(v/10000), time.Month(v%10000/100), int(v%100), 0, 0, 0, 0, time.Local)
}

// Decode 解析交易时段响应。
func (macServerSession) Decode(bs []byte) (*MacServerSession, error) {
	if len(bs) < 79 {
		return nil, errors.New("数据长度不足")
	}
	resp := &MacServerSession{}
	pos := 2 + 8 + 3 + 9 //count(H) + flags(8) + tag(3) + reserved(9)
	resp.Today = macDate(Uint32(bs[pos : pos+4]))
	pos += 4 + 4 //ts1
	for i := 0; i < 4; i++ {
		resp.Sessions1[i] = MacSession{
			Open:  int(Uint16(bs[pos+i*4 : pos+i*4+2])),
			Close: int(Uint16(bs[pos+i*4+2 : pos+i*4+4])),
		}
	}
	pos += 16
	for i := 0; i < 4; i++ {
		resp.Sessions2[i] = MacSession{
			Open:  int(Uint16(bs[pos+i*4 : pos+i*4+2])),
			Close: int(Uint16(bs[pos+i*4+2 : pos+i*4+4])),
		}
	}
	pos += 16 + 1 //flag
	resp.LastTradingDay = macDate(Uint32(bs[pos : pos+4]))
	pos += 4 + 4 //ts2
	if len(bs) >= pos+8 {
		resp.MarketParam1 = Uint32(bs[pos : pos+4])
		resp.MarketParam2 = Uint32(bs[pos+4 : pos+8])
	}
	return resp, nil
}

type macKlineCount struct{}

// Frame 构造 K线总量查询请求(offset 固定 0)。
func (macKlineCount) Frame() *Frame {
	body := make([]byte, 4+4+5)
	//offset=0, count=1(占位, 服务器仅返回总量)
	body[8] = 0x01
	return &Frame{
		Prefix:  PrefixMac,
		Control: Control01,
		Type:    TypeMacKlineOffset,
		Data:    body,
	}
}

// MacKlineCount K线数据总量。
type MacKlineCount struct {
	Total    uint32 //K线总量(大端序, 协议特例)
	Returned uint32 //本次返回数量
}

func (this *MacKlineCount) String() string {
	return fmt.Sprintf("K线总量 %d, 本次返回 %d", this.Total, this.Returned)
}

// Decode 解析 K线总量响应。注意 total 为大端序。
func (macKlineCount) Decode(bs []byte) (*MacKlineCount, error) {
	if len(bs) < 8 {
		return nil, errors.New("数据长度不足")
	}
	return &MacKlineCount{
		Total:    uint32(bs[0])<<24 | uint32(bs[1])<<16 | uint32(bs[2])<<8 | uint32(bs[3]),
		Returned: Uint32(bs[4:8]),
	}, nil
}

// MacFileMeta 远程文件元信息。
type MacFileMeta struct {
	Offset int64  //文件偏移(请求回显)
	Size   int64  //文件大小(字节)
	Flag   int8   //标志(含义未知)
	Hash   string //32 位哈希(ascii)
}

type macFileMeta struct{}

// macFileNameBytes 文件名编码: 70B 名字 + 30B 保留
func macFileNameBytes(name string) []byte {
	b := make([]byte, 100)
	copy(b, name)
	return b
}

// Frame 构造文件元信息查询请求。
func (macFileMeta) Frame(name string, offset uint32) *Frame {
	body := make([]byte, 4+100)
	body[0] = byte(offset)
	body[1] = byte(offset >> 8)
	body[2] = byte(offset >> 16)
	body[3] = byte(offset >> 24)
	copy(body[4:], macFileNameBytes(name))
	return &Frame{
		Prefix:  PrefixMac,
		Control: Control01,
		Type:    TypeMacFileMeta,
		Data:    body,
	}
}

// Decode 解析文件元信息响应。
func (macFileMeta) Decode(bs []byte) (*MacFileMeta, error) {
	if len(bs) < 41 {
		return nil, errors.New("数据长度不足")
	}
	return &MacFileMeta{
		Offset: int64(Uint32(bs[0:4])),
		Size:   int64(Uint32(bs[4:8])),
		Flag:   int8(bs[8]),
		Hash:   trimZero(string(bs[9:41])),
	}, nil
}

func trimZero(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return s[:i]
		}
	}
	return s
}

type macFileData struct{}

// Frame 构造文件分段下载请求。
func (macFileData) Frame(name string, index, offset, size uint32) *Frame {
	body := make([]byte, 12+100)
	body[0] = byte(index)
	body[1] = byte(index >> 8)
	body[2] = byte(index >> 16)
	body[3] = byte(index >> 24)
	body[4] = byte(offset)
	body[5] = byte(offset >> 8)
	body[6] = byte(offset >> 16)
	body[7] = byte(offset >> 24)
	body[8] = byte(size)
	body[9] = byte(size >> 8)
	body[10] = byte(size >> 16)
	body[11] = byte(size >> 24)
	copy(body[12:], macFileNameBytes(name))
	return &Frame{
		Prefix:  PrefixMac,
		Control: Control01,
		Type:    TypeMacFileData,
		Data:    body,
	}
}

// Decode 解析文件数据响应(前 8B 保留, 之后为文件内容)。
func (macFileData) Decode(bs []byte) ([]byte, error) {
	if len(bs) < 8 {
		return nil, errors.New("数据长度不足")
	}
	return bs[8:], nil
}
