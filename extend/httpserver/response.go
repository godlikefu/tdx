package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/injoyai/tdx/protocol"
)

// Response 统一响应结构
type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// respondOK 成功响应
func respondOK(w http.ResponseWriter, data any) {
	respondJSON(w, http.StatusOK, Response{Code: 0, Msg: "ok", Data: data})
}

// respondErr 错误响应
func respondErr(w http.ResponseWriter, httpCode int, msg string) {
	respondJSON(w, httpCode, Response{Code: 1, Msg: msg, Data: nil})
}

// respondJSON 统一缓冲输出: 先整体序列化, 再单次写入并显式声明 Content-Length。
// 流式写入(Encoder 直写 w)在异常/客户端断开时会让客户端拿到静默截断的 JSON
// ("Unexpected end of JSON input"); 缓冲+定长使响应原子化, 截断必可被客户端检测。
func respondJSON(w http.ResponseWriter, httpCode int, resp Response) {
	bs, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(bs)))
	w.WriteHeader(httpCode)
	if n, werr := w.Write(bs); werr != nil || n != len(bs) {
		// 客户端提前断开等; 已无法补救, 仅占位说明
		_ = werr
	}
}

// ---- 参数解析辅助 ----

func queryStr(r *http.Request, key string) (string, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return "", fmt.Errorf("参数 %s 不能为空", key)
	}
	return v, nil
}

func queryUint8(r *http.Request, key string) (uint8, error) {
	s, err := queryStr(r, key)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("参数 %s 格式错误: %s", key, s)
	}
	return uint8(n), nil
}

func queryUint16(r *http.Request, key string) (uint16, error) {
	s, err := queryStr(r, key)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("参数 %s 格式错误: %s", key, s)
	}
	return uint16(n), nil
}

func queryUint32(r *http.Request, key string) (uint32, error) {
	s, err := queryStr(r, key)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("参数 %s 格式错误: %s", key, s)
	}
	return uint32(n), nil
}

// queryUint16Default 解析 uint16,为空时返回默认值
func queryUint16Default(r *http.Request, key string, def uint16) uint16 {
	s := r.URL.Query().Get(key)
	if s == "" {
		return def
	}
	n, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return def
	}
	return uint16(n)
}

// parseExchange 将字符串转换为 protocol.Exchange
func parseExchange(s string) (protocol.Exchange, error) {
	switch s {
	case "sh", "SH":
		return protocol.ExchangeSH, nil
	case "sz", "SZ":
		return protocol.ExchangeSZ, nil
	case "bj", "BJ":
		return protocol.ExchangeBJ, nil
	default:
		return 0, fmt.Errorf("不支持的交易所: %s (可选: sh, sz, bj)", s)
	}
}
