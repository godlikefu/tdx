package protocol

import (
	"encoding/binary"
	"math"
	"testing"
)

// TestMacTradeFrame 测试 mac 逐笔成交请求帧结构
func TestMacTradeFrame(t *testing.T) {
	f, err := MMacTrade.Frame("sh601872", 20260828, 1000, 500)
	if err != nil {
		t.Fatal(err)
	}
	bs := f.Bytes()

	// 帧头: 0x1C + MsgID(由 SendFrame 填充,此处为0) + version + 长度×2 + 命令号
	if bs[0] != PrefixMac {
		t.Errorf("Prefix = %#x, 期望 %#x", bs[0], PrefixMac)
	}
	if got := binary.LittleEndian.Uint16(bs[10:12]); got != TypeMacTransaction {
		t.Errorf("Type = %#x, 期望 %#x", got, TypeMacTransaction)
	}
	length := int(binary.LittleEndian.Uint16(bs[6:8]))
	if length != len(bs)-10 {
		t.Errorf("长度字段 %d, 实际 body+2 = %d", length, len(bs)-10)
	}

	// 请求体: market(H) + code(22B) + ymd(I) + start(I) + count(H) + 10B
	body := bs[12:]
	if m := binary.LittleEndian.Uint16(body[0:2]); m != 1 {
		t.Errorf("market = %d, 期望 1(SH)", m)
	}
	if string(body[2:8]) != "601872" {
		t.Errorf("code = %q, 期望 601872", body[2:8])
	}
	if ymd := binary.LittleEndian.Uint32(body[24:28]); ymd != 20260828 {
		t.Errorf("ymd = %d, 期望 20260828", ymd)
	}
	if start := binary.LittleEndian.Uint32(body[28:32]); start != 1000 {
		t.Errorf("start = %d, 期望 1000", start)
	}
	if count := binary.LittleEndian.Uint16(body[32:34]); count != 500 {
		t.Errorf("count = %d, 期望 500", count)
	}
	for i, b := range body[34:] {
		if b != 0 {
			t.Errorf("尾部保留字节[%d] = %#x, 期望 0", i, b)
		}
	}

	// 裸代码自动推断
	if _, err := MMacTrade.Frame("601872", 0, 0, 10); err != nil {
		t.Errorf("裸代码 601872 应可用: %v", err)
	}
	if _, err := MMacTrade.Frame("sz000001", 0, 0, 10); err != nil {
		t.Errorf("sz000001 应可用: %v", err)
	}
	if _, err := MMacTrade.Frame("", 0, 0, 10); err == nil {
		t.Error("空代码应报错")
	}
}

// TestMacTradeDecode 测试 mac 逐笔成交响应解析
func TestMacTradeDecode(t *testing.T) {
	// 构造合成响应: 39字节头 + 3条18字节记录
	// 头: market(2) + code(22) + ymd(4) + flag(1)@28 + count(2)@29 + start(4)@31 + total(4)@35
	bs := make([]byte, 39+3*18)
	copy(bs[2:24], "601872")
	binary.LittleEndian.PutUint32(bs[24:28], 20260828)
	bs[28] = 0x01                                  //flag
	binary.LittleEndian.PutUint16(bs[29:31], 3)    //count
	binary.LittleEndian.PutUint32(bs[31:35], 1000) //start
	binary.LittleEndian.PutUint32(bs[35:39], 4597) //total

	// 记录1: 09:25:01, 19.00元, 7907手, 626笔, 中性(2)
	rec := bs[39:]
	binary.LittleEndian.PutUint32(rec[0:4], 9*3600+25*60+1)
	binary.LittleEndian.PutUint32(rec[4:8], math.Float32bits(19.00))
	binary.LittleEndian.PutUint32(rec[8:12], 7907)
	binary.LittleEndian.PutUint32(rec[12:16], 626)
	binary.LittleEndian.PutUint16(rec[16:18], 2)

	// 记录2: 14:57:00, 18.98元, 9手, 2笔, 买入(0)
	rec = bs[39+18:]
	binary.LittleEndian.PutUint32(rec[0:4], 14*3600+57*60)
	binary.LittleEndian.PutUint32(rec[4:8], math.Float32bits(18.98))
	binary.LittleEndian.PutUint32(rec[8:12], 9)
	binary.LittleEndian.PutUint32(rec[12:16], 2)
	binary.LittleEndian.PutUint16(rec[16:18], 0)

	// 记录3: 15:16:34, 18.98元, 3手, 1笔, 盘后(5)
	rec = bs[39+36:]
	binary.LittleEndian.PutUint32(rec[0:4], 15*3600+16*60+34)
	binary.LittleEndian.PutUint32(rec[4:8], math.Float32bits(18.98))
	binary.LittleEndian.PutUint32(rec[8:12], 3)
	binary.LittleEndian.PutUint32(rec[12:16], 1)
	binary.LittleEndian.PutUint16(rec[16:18], 5)

	resp, err := MMacTrade.Decode(bs, MacTradeCache{Date: "20260828", Code: "sh601872"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 3 {
		t.Errorf("Count = %d, 期望 3", resp.Count)
	}
	if resp.Total != 4597 {
		t.Errorf("Total = %d, 期望 4597", resp.Total)
	}
	if len(resp.List) != 3 {
		t.Fatalf("len(List) = %d, 期望 3", len(resp.List))
	}

	r1 := resp.List[0]
	if got := r1.Time.Format("15:04:05"); got != "09:25:01" {
		t.Errorf("Time = %s, 期望 09:25:01", got)
	}
	if r1.Price != 19000 { //厘
		t.Errorf("Price = %d, 期望 19000", r1.Price)
	}
	if r1.Volume != 7907 {
		t.Errorf("Volume = %d, 期望 7907", r1.Volume)
	}
	if r1.TradeCount != 626 {
		t.Errorf("TradeCount = %d, 期望 626", r1.TradeCount)
	}
	if r1.Status != 2 || r1.StatusString() != "" {
		t.Errorf("Status = %d(%q), 期望 2(中性无描述)", r1.Status, r1.StatusString())
	}

	r3 := resp.List[2]
	if got := r3.Time.Format("15:04:05"); got != "15:16:34" {
		t.Errorf("Time = %s, 期望 15:16:34", got)
	}
	if r3.Status != 5 || r3.StatusString() != "盘后" {
		t.Errorf("Status = %d(%q), 期望 5(盘后)", r3.Status, r3.StatusString())
	}
	if r3.String() == "" {
		t.Error("String() 不应为空")
	}
}

// TestMacTradeDecodeError 测试异常输入
func TestMacTradeDecodeError(t *testing.T) {
	if _, err := MMacTrade.Decode(make([]byte, 10), MacTradeCache{Date: "20260828"}); err == nil {
		t.Error("数据过短应报错")
	}
	// count 与实际长度不符
	bs := make([]byte, 39)
	binary.LittleEndian.PutUint16(bs[29:31], 5)
	if _, err := MMacTrade.Decode(bs, MacTradeCache{Date: "20260828"}); err == nil {
		t.Error("count 与长度不符应报错")
	}
	// 非法日期
	if _, err := MMacTrade.Decode(make([]byte, 39+18), MacTradeCache{Date: "bad"}); err == nil {
		t.Error("非法日期应报错")
	}
}
