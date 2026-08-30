package protocol

import (
	"encoding/binary"
	"math"
	"testing"
)

// TestMacQuoteFrame 测试 mac 批量报价请求帧
func TestMacQuoteFrame(t *testing.T) {
	f, err := MMacQuote.Frame([]string{"sh601872", "sz000001"})
	if err != nil {
		t.Fatal(err)
	}
	bs := f.Bytes()

	if bs[0] != PrefixMac {
		t.Errorf("Prefix = %#x, 期望 %#x", bs[0], PrefixMac)
	}
	if got := binary.LittleEndian.Uint16(bs[10:12]); got != TypeMacQuote {
		t.Errorf("Type = %#x, 期望 %#x", got, TypeMacQuote)
	}

	body := bs[12:]
	// 位图: 20字节, 校验已请求字段位
	if len(body) < 22 {
		t.Fatalf("body 过短: %d", len(body))
	}
	for _, fd := range macQuoteFields {
		if body[fd.bit/8]&(1<<(fd.bit%8)) == 0 {
			t.Errorf("位图缺少字段 %#x", fd.bit)
		}
	}
	// 位图高 4 字节(位 128-159)应恰好覆盖 0x80+ 盘口字段位(0x80-0x8B)
	if body[16] != 0xff || body[17] != 0x0f {
		t.Errorf("位图高字节 = %#02x %#02x, 期望 0xff 0x0f", body[16], body[17])
	}
	if body[18] != 0 || body[19] != 0 {
		t.Errorf("位图 18/19 字节应为 0: %#02x %#02x", body[18], body[19])
	}
	// 数量
	if n := binary.LittleEndian.Uint16(body[20:22]); n != 2 {
		t.Errorf("count = %d, 期望 2", n)
	}
	// 第一只: sh601872
	if m := binary.LittleEndian.Uint16(body[22:24]); m != 1 {
		t.Errorf("market = %d, 期望 1", m)
	}
	if string(body[24:30]) != "601872" {
		t.Errorf("code = %q, 期望 601872", body[24:30])
	}
	// 第二只: sz000001
	off := 22 + 24
	if m := binary.LittleEndian.Uint16(body[off : off+2]); m != 0 {
		t.Errorf("market = %d, 期望 0", m)
	}
	if string(body[off+2:off+8]) != "000001" {
		t.Errorf("code = %q, 期望 000001", body[off+2:off+8])
	}

	if _, err := MMacQuote.Frame(nil); err == nil {
		t.Error("空代码列表应报错")
	}
}

// TestMacQuoteDecode 测试 mac 批量报价响应解析
func TestMacQuoteDecode(t *testing.T) {
	fields := macQuoteActiveFields(macQuoteBitmap, macQuoteFields)
	rowLen := 68 + 4*len(fields)

	// 响应: 20B 位图回显 + total(4) + rowCount(2) + 2 行
	bs := make([]byte, 26+2*rowLen)
	copy(bs[:20], macQuoteBitmap)
	binary.LittleEndian.PutUint32(bs[20:24], 2) //total
	binary.LittleEndian.PutUint16(bs[24:26], 2) //rowCount

	// 行1: sh601872 金风科技
	row := bs[26:]
	binary.LittleEndian.PutUint16(row[0:2], 1)
	copy(row[2:24], "601872")
	copy(row[24:68], []byte("GOLDWIND")) //GBK 数字/字母与 UTF-8 同编码, 中文名需真实 GBK 字节
	for i, fd := range fields {
		v := row[68+i*4 : 68+i*4+4]
		switch fd.bit {
		case mqClose:
			binary.LittleEndian.PutUint32(v, math.Float32bits(19.00))
		case mqVolume:
			binary.LittleEndian.PutUint32(v, 859642)
		case mqMainNetAmount:
			binary.LittleEndian.PutUint32(v, math.Float32bits(-88180000))
		default:
			binary.LittleEndian.PutUint32(v, math.Float32bits(float32(fd.bit))) //可识别的占位值
		}
	}

	// 行2: sz000001
	row = bs[26+rowLen:]
	binary.LittleEndian.PutUint16(row[0:2], 0)
	copy(row[2:24], "000001")
	copy(row[24:68], []byte("PINGAN"))

	list, err := MMacQuote.Decode(bs)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, 期望 2", len(list))
	}

	q := list[0]
	if q.Code != "601872" || q.Name != "GOLDWIND" || q.Market != 1 {
		t.Errorf("头部不符: %+v", q)
	}
	if q.Price != 19000 { //厘
		t.Errorf("Price = %d, 期望 19000", q.Price)
	}
	if q.Volume != 859642 {
		t.Errorf("Volume = %d, 期望 859642", q.Volume)
	}
	if q.MainNetAmount != -88180000 {
		t.Errorf("MainNetAmount = %f, 期望 -88180000", q.MainNetAmount)
	}
	// 每个字段位都应至少映射到结构体(以 PreClose 为例: 占位值 float32(0)=0 厘)
	if q.PreClose != 0 {
		t.Errorf("PreClose = %d, 期望 0", q.PreClose)
	}

	q2 := list[1]
	if q2.Code != "000001" || q2.Market != 0 {
		t.Errorf("行2 头部不符: %+v", q2)
	}
}

// TestMacQuoteFieldsSorted 校验字段表按 bit 升序(与响应字段排列顺序一致的前提)
func TestMacQuoteFieldsSorted(t *testing.T) {
	for i := 1; i < len(macQuoteFields); i++ {
		if macQuoteFields[i].bit <= macQuoteFields[i-1].bit {
			t.Errorf("字段表未按 bit 升序: %#x 在 %#x 之后", macQuoteFields[i].bit, macQuoteFields[i-1].bit)
		}
	}
	// 位图回显应与请求一致(自洽)
	// 16字节位图中每个字段位都应置位
	for _, fd := range macQuoteFields {
		if macQuoteBitmap[fd.bit/8]&(1<<(fd.bit%8)) == 0 {
			t.Errorf("位图缺少 %#x", fd.bit)
		}
	}
}
