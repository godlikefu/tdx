package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
)

/*
mac JSON 查询命令(0x1218)。

同一命令号用帧头 head_flag(Prefix 字节)区分子命令:
	head=1: 个股所属板块(Stock_GLHQ)
	head=2: 个股资金流向(Stock_ZJLX)
响应为 27 字节头(market H + 12s 查询信息 + 5B 保留 + 8s ext) + GBK 编码的 JSON 数组。
逆向来源: pytdx / easy_tdx mac/commands/symbol_belong_board.py + symbol_capital_flow.py。
*/

var (
	MMacCapitalFlow = macCapitalFlow{}
	MMacBelongBoard = macBelongBoard{}
)

// MacCapitalFlowCache / MacBelongBoardCache 0x1218 响应分发标识:
// 同命令号(head 区分)的两个子命令, 请求时通过 SendFrame 缓存携带, 供响应按类型分发。
type (
	MacCapitalFlowCache struct{}
	MacBelongBoardCache struct{}
)

// mac JSON 查询请求体: market(H) + code(8B) + 16B 保留 + label(21B)
func macQueryFrame(headFlag byte, code string, label string) (*Frame, error) {
	exchange, number, err := DecodeCode(code)
	if err != nil {
		return nil, err
	}
	body := make([]byte, 2+8+16+21)
	body[0] = byte(exchange.Uint8())
	body[1] = 0x00
	copy(body[2:10], number)
	copy(body[26:47], label)
	return &Frame{
		Prefix:  headFlag, //head=1/2 区分子命令
		Control: Control01,
		Type:    TypeMacQuery,
		Data:    body,
	}, nil
}

// macJSONResult 解析 0x1218 响应: 27 字节头 + GBK JSON
func macJSONResult(bs []byte) ([][]any, error) {
	if len(bs) < 27 {
		return nil, errors.New("数据长度不足")
	}
	var result [][]any
	if err := json.Unmarshal(UTF8ToGBK(bs[27:]), &result); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	return result, nil
}

// jsonFloat JSON 值 → float64(数字直接转, 字符串尝试解析)
func jsonFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		var f float64
		_, err := fmt.Sscanf(t, "%f", &f)
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}

// jsonInt JSON 值 → int64
func jsonInt(v any) int64 {
	return int64(jsonFloat(v))
}

// jsonStr JSON 值 → string(数字转字符串)
func jsonStr(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%.0f", t)
	default:
		return ""
	}
}

// MacCapitalFlow 个股资金流向(实时, 通达信口径)。
type MacCapitalFlow struct {
	MainIn   float64   //今日主力流入(元)
	MainOut  float64   //今日主力流出(元)
	MainNet  float64   //今日主力净流入(元) = 主入-主出
	SmallIn  float64   //今日散户流入(元)
	SmallOut float64   //今日散户流出(元)
	SmallNet float64   //今日散户净流入(元) = 散入-散出
	FiveDay  []float64 //五日数据原始数组(口径未完全确定, 疑为 [买5d,卖5d,超大单,大单,中单,小单])
}

func (this *MacCapitalFlow) String() string {
	return fmt.Sprintf("主力净流入 %.0f(元) 散户净流入 %.0f(元)", this.MainNet, this.SmallNet)
}

type macCapitalFlow struct{}

// Frame 构造资金流向请求(0x1218 head=2)。
func (macCapitalFlow) Frame(code string) (*Frame, error) {
	return macQueryFrame(2, code, "Stock_ZJLX")
}

// Decode 解析资金流向响应。
// JSON: [[主入,主出,散入,散出], [五日6项...]]
func (macCapitalFlow) Decode(bs []byte) (*MacCapitalFlow, error) {
	result, err := macJSONResult(bs)
	if err != nil {
		return nil, err
	}
	resp := &MacCapitalFlow{}
	if len(result) > 0 {
		today := result[0]
		if len(today) > 0 {
			resp.MainIn = jsonFloat(today[0])
		}
		if len(today) > 1 {
			resp.MainOut = jsonFloat(today[1])
		}
		if len(today) > 2 {
			resp.SmallIn = jsonFloat(today[2])
		}
		if len(today) > 3 {
			resp.SmallOut = jsonFloat(today[3])
		}
		resp.MainNet = resp.MainIn - resp.MainOut
		resp.SmallNet = resp.SmallIn - resp.SmallOut
	}
	if len(result) > 1 {
		for _, v := range result[1] {
			resp.FiveDay = append(resp.FiveDay, jsonFloat(v))
		}
	}
	return resp, nil
}

// MacBelongBoard 个股所属板块。
type MacBelongBoard struct {
	BoardType int64   //板块类型
	Market    int64   //板块市场
	BoardCode string  //板块代码
	BoardName string  //板块名称
	Close     float64 //最新价
	PreClose  float64 //昨收
}

func (this *MacBelongBoard) String() string {
	return fmt.Sprintf("%s(%s) 类型%d 市场%d", this.BoardName, this.BoardCode, this.BoardType, this.Market)
}

type macBelongBoard struct{}

// Frame 构造所属板块请求(0x1218 head=1)。
func (macBelongBoard) Frame(code string) (*Frame, error) {
	return macQueryFrame(1, code, "Stock_GLHQ")
}

// Decode 解析所属板块响应。
// JSON 行: [板块类型, 板块市场, 板块代码, 板块名称, 现价, 昨收] (9或13列, 后段忽略)
func (macBelongBoard) Decode(bs []byte) ([]*MacBelongBoard, error) {
	result, err := macJSONResult(bs)
	if err != nil {
		return nil, err
	}
	out := make([]*MacBelongBoard, 0, len(result))
	for _, row := range result {
		if len(row) < 6 {
			continue
		}
		out = append(out, &MacBelongBoard{
			BoardType: jsonInt(row[0]),
			Market:    jsonInt(row[1]),
			BoardCode: jsonStr(row[2]),
			BoardName: jsonStr(row[3]),
			Close:     jsonFloat(row[4]),
			PreClose:  jsonFloat(row[5]),
		})
	}
	return out, nil
}
