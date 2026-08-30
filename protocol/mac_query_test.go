package protocol

import (
	"encoding/binary"
	"math"
	"testing"
)

// TestMacCapitalFlowDecode 测试资金流向 JSON 响应解析(GBK 编码的 JSON)
func TestMacCapitalFlowDecode(t *testing.T) {
	// 构造 GBK JSON: [[1000000,-2000000,300000,-400000],[5,6,7,8,9,10]]
	// (纯数字/ASCII JSON 与 GBK 同编码)
	js := `[[1000000,-2000000,300000,-400000],[5,6,7,8,9,10]]`
	body := make([]byte, 27+len(js))
	binary.LittleEndian.PutUint16(body[0:2], 1) //market
	copy(body[27:], js)

	resp, err := MMacCapitalFlow.Decode(body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.MainIn != 1000000 || resp.MainOut != -2000000 {
		t.Errorf("MainIn/Out = %f/%f, 期望 1000000/-2000000", resp.MainIn, resp.MainOut)
	}
	if resp.MainNet != 3000000 {
		t.Errorf("MainNet = %f, 期望 3000000", resp.MainNet)
	}
	if resp.SmallIn != 300000 || resp.SmallOut != -400000 || resp.SmallNet != 700000 {
		t.Errorf("SmallIn/Out/Net = %f/%f/%f", resp.SmallIn, resp.SmallOut, resp.SmallNet)
	}
	if len(resp.FiveDay) != 6 || resp.FiveDay[5] != 10 {
		t.Errorf("FiveDay = %v", resp.FiveDay)
	}

	if _, err := MMacCapitalFlow.Decode(make([]byte, 10)); err == nil {
		t.Error("数据过短应报错")
	}
}

// TestMacCapitalFlowGBKJSON 测试 GBK 中文 JSON(板块名)解码
func TestMacCapitalFlowGBKJSON(t *testing.T) {
	// "航运" 的 GBK 编码: B4 AC D4 CB
	js := "[[1,1,\"881160\",\"\xB4\xAC\xD4\xCB\",10.5,9.8]]"
	body := make([]byte, 27+len(js))
	copy(body[27:], js)

	boards, err := MMacBelongBoard.Decode(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(boards) != 1 {
		t.Fatalf("len = %d, 期望 1", len(boards))
	}
	b := boards[0]
	if b.BoardCode != "881160" || b.BoardName != "船运" {
		t.Errorf("BoardCode/Name = %q/%q, 期望 881160/船运", b.BoardCode, b.BoardName)
	}
	if b.Close != 10.5 || b.PreClose != 9.8 {
		t.Errorf("Close/PreClose = %f/%f", b.Close, b.PreClose)
	}
}

// TestMacBoardMembersFrame 测试板块成分请求帧
func TestMacBoardMembersFrame(t *testing.T) {
	f, err := MMacBoardMembers.Frame("881160", MacSortByChangePct, 20, 80, MacSortDesc)
	if err != nil {
		t.Fatal(err)
	}
	bs := f.Bytes()
	if got := binary.LittleEndian.Uint16(bs[10:12]); got != TypeMacBoardMembers {
		t.Errorf("Type = %#x, 期望 %#x", got, TypeMacBoardMembers)
	}
	body := bs[12:]
	// 板块代码转换: 881160 → 21160 (88开头: N-880000+20000)
	if code := binary.LittleEndian.Uint32(body[0:4]); code != 21160 {
		t.Errorf("board = %d, 期望 21160", code)
	}
	// 排序字段 0x0E, start 20, count 80, order 1
	if st := binary.LittleEndian.Uint16(body[13:15]); st != MacSortByChangePct {
		t.Errorf("sortType = %#x", st)
	}
	if st := binary.LittleEndian.Uint32(body[15:19]); st != 20 {
		t.Errorf("start = %d", st)
	}
	if c := binary.LittleEndian.Uint16(body[19:21]); c != 80 {
		t.Errorf("count = %d", c)
	}
	if body[21] != MacSortDesc {
		t.Errorf("sortOrder = %d", body[21])
	}
	// 控制区 byte3 = 1(扩展模式)
	if body[42] != 1 {
		t.Errorf("control byte3 = %d, 期望 1", body[42])
	}
	// 代码转换规则
	cases := map[string]uint32{"880216": 20216, "399372": 30372, "899050": 32050, "000686": 31686}
	for in, want := range cases {
		got, err := convertMacBoardCode(in)
		if err != nil || got != want {
			t.Errorf("convertMacBoardCode(%q) = %d, %v; 期望 %d", in, got, err, want)
		}
	}
	if _, err := MMacBoardMembers.Frame("abc", 0, 0, 10, 0); err == nil {
		t.Error("非数字板块代码应报错")
	}
}

// TestMacBoardMembersDecode 测试板块成分响应(行格式同 0x122B, 字段集不同)
func TestMacBoardMembersDecode(t *testing.T) {
	fields := macQuoteActiveFields(macBoardMembersBitmap, macBoardMemberFields)
	rowLen := 68 + 4*len(fields)

	bs := make([]byte, 26+rowLen)
	copy(bs[:16], macBoardMembersBitmap[:16])
	binary.LittleEndian.PutUint32(bs[20:24], 1)
	binary.LittleEndian.PutUint16(bs[24:26], 1)

	row := bs[26:]
	binary.LittleEndian.PutUint16(row[0:2], 1)
	copy(row[2:24], "600028")
	copy(row[24:68], []byte("TEST"))
	for i, fd := range fields {
		v := row[68+i*4 : 68+i*4+4]
		switch fd.bit {
		case mqClose:
			binary.LittleEndian.PutUint32(v, math.Float32bits(6.66))
		case mqVolume:
			binary.LittleEndian.PutUint32(v, 123456)
		}
	}

	list, err := MMacBoardMembers.Decode(bs)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, 期望 1", len(list))
	}
	q := list[0]
	if q.Code != "600028" || q.Price != 6660 || q.Volume != 123456 {
		t.Errorf("解析结果: %+v", q)
	}
}

// TestMacServerSessionDecode 测试交易时段响应解析
func TestMacServerSessionDecode(t *testing.T) {
	bs := make([]byte, 80)
	binary.LittleEndian.PutUint16(bs[0:2], 1) //count
	// pos 22: today 20260828
	binary.LittleEndian.PutUint32(bs[22:26], 20260828)
	// pos 30: sessions1 = 09:30-11:30 (570,690), 13:00-15:00 (780,900)
	binary.LittleEndian.PutUint16(bs[30:32], 570)
	binary.LittleEndian.PutUint16(bs[32:34], 690)
	binary.LittleEndian.PutUint16(bs[34:36], 780)
	binary.LittleEndian.PutUint16(bs[36:38], 900)
	// pos 46: sessions2 同样填一组
	copy(bs[46:62], bs[30:46])
	// pos 63: last trading day 20260828
	binary.LittleEndian.PutUint32(bs[63:67], 20260828)

	resp, err := MMacServerSession.Decode(bs)
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Today.Format("20060102"); got != "20260828" {
		t.Errorf("Today = %s, 期望 20260828", got)
	}
	if got := resp.LastTradingDay.Format("20060102"); got != "20260828" {
		t.Errorf("LastTradingDay = %s", got)
	}
	s := resp.Sessions1[0]
	if s.Open != 570 || s.Close != 690 || s.String() != "09:30-11:30" {
		t.Errorf("Sessions1[0] = %s", s)
	}
	if resp.Sessions1[1].Open != 780 || resp.Sessions1[1].Close != 900 {
		t.Errorf("Sessions1[1] = %s", resp.Sessions1[1])
	}
}

// TestMacKlineCountDecode 测试 K线总量解析(total 大端序特例)
func TestMacKlineCountDecode(t *testing.T) {
	bs := make([]byte, 8)
	// total 大端: 0x01234567 = 19088743
	bs[0], bs[1], bs[2], bs[3] = 0x01, 0x23, 0x45, 0x67
	binary.LittleEndian.PutUint32(bs[4:8], 1)

	resp, err := MMacKlineCount.Decode(bs)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 0x01234567 {
		t.Errorf("Total = %d, 期望 %d", resp.Total, 0x01234567)
	}
	if resp.Returned != 1 {
		t.Errorf("Returned = %d", resp.Returned)
	}
}

// TestMacFileMetaDecode 测试文件元信息解析
func TestMacFileMetaDecode(t *testing.T) {
	bs := make([]byte, 41+10)
	binary.LittleEndian.PutUint32(bs[0:4], 0)     //offset
	binary.LittleEndian.PutUint32(bs[4:8], 12345) //size
	bs[8] = 0x02                                  //flag
	copy(bs[9:41], "d41d8cd98f00b204e9800998ecf8427e")

	meta, err := MMacFileMeta.Decode(bs)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Size != 12345 || meta.Flag != 2 {
		t.Errorf("Size/Flag = %d/%d", meta.Size, meta.Flag)
	}
	if meta.Hash != "d41d8cd98f00b204e9800998ecf8427e" {
		t.Errorf("Hash = %q", meta.Hash)
	}

	// 文件数据: 前 8B 保留
	data, err := MMacFileData.Decode(make([]byte, 10))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 2 {
		t.Errorf("data len = %d, 期望 2", len(data))
	}
}
