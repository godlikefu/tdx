package protocol

import "time"

const (
	TypeConnect            = 0x000D //建立连接
	TypeHeart              = 0x0004 //心跳
	TypeGbbq               = 0x000F //除权除息
	TypeCount              = 0x044E //获取股票数量
	TypeCode               = 0x0450 //获取股票代码
	TypeQuote              = 0x053E //行情信息
	TypeMinute             = 0x051D //分时数据
	TypeCallAuction        = 0x056A //集合竞价
	TypeMinuteTrade        = 0x0FC5 //分时交易
	TypeHistoryMinute      = 0x0FB4 //历史分时数据
	TypeHistoryMinuteTrade = 0x0FB5 //历史分时交易
	TypeKline              = 0x052D //K线图
	TypeLogin2             = 0x0FDB //第二次登录(标准协议登录命令;mac 方言握手第三条的响应类型)
	TypeMacTransaction     = 0x122F //mac 方言:逐笔成交(秒级时间+成交笔数,含盘后固定价格成交)
	TypeMacQuote           = 0x122B //mac 方言:批量自定义字段报价(主力净流入/内外盘/涨跌停价等, 实时)
	TypeMacBoardMembers    = 0x122C //mac 方言:板块成分报价(响应格式同 0x122B)
	TypeMacQuery           = 0x1218 //mac 方言:JSON查询(帧头 head=1 所属板块 / head=2 资金流向, 按缓存类型区分)
	TypeMacServerSession   = 0x120F //mac 方言:服务器交易时段
	TypeMacKlineOffset     = 0x124A //mac 方言:K线数据偏移/总量
	TypeMacFileMeta        = 0x1215 //mac 方言:远程文件元信息查询
	TypeMacFileData        = 0x1217 //mac 方言:远程文件分段下载
)

var (
	// ExchangeEstablish 交易所成立时间
	ExchangeEstablish = time.Date(1990, 12, 19, 0, 0, 0, 0, time.Local)
)

/*
从其他地方复制
const (
	LOGIN_ONE       = 0x000d //第一次登录
	LOGIN_TWO       = 0x0fdb //第二次登录
	HEART           = 0x0004 //心跳维持
	STOCK_COUNT     = 0x044e //股票数目
	STOCK_LIST      = 0x0450 //股票列表
	KMINUTE         = 0x0537 //当天分时K线
	KMINUTE_OLD     = 0x0fb4 //指定日期分时K线
	KLINE           = 0x052d //股票K线
	BIDD            = 0x056a //当日的竞价
	QUOTE           = 0x053e //实时五笔报价
	QUOTE_SORT      = 0x053e //沪深排序
	TRANSACTION     = 0x0fc5 //分笔成交明细
	TRANSACTION_OLD = 0x0fb5 //历史分笔成交明细
	FINANCE         = 0x0010 //财务数据
	COMPANY         = 0x02d0 //公司数据  F10
	EXDIVIDEND      = 0x000f //除权除息
	FILE_DIRECTORY  = 0x02cf //公司文件目录
	FILE_CONTENT    = 0x02d0 //公司文件内容
)

const (
	KMSG_CMD1                   = 0x000d // 建立链接
	KMSG_CMD2                   = 0x0fdb // 建立链接
	KMSG_PING                   = 0x0015 // 测试连接
	KMSG_HEARTBEAT              = 0xFFFF // 心跳(自定义)
	KMSG_SECURITYCOUNT          = 0x044e // 证券数量
	KMSG_BLOCKINFOMETA          = 0x02c5 // 板块文件信息
	KMSG_BLOCKINFO              = 0x06b9 // 板块文件
	KMSG_COMPANYCATEGORY        = 0x02cf // 公司信息文件信息
	KMSG_COMPANYCONTENT         = 0x02d0 // 公司信息描述
	KMSG_FINANCEINFO            = 0x0010 // 财务信息
	KMSG_HISTORYMINUTETIMEDATE  = 0x0fb4 // 历史分时信息
	KMSG_HISTORYTRANSACTIONDATA = 0x0fb5 // 历史分笔成交信息
	KMSG_INDEXBARS              = 0x052d // 指数K线
	KMSG_SECURITYBARS           = 0x052d // 股票K线
	KMSG_MINUTETIMEDATA         = 0x0537 // 分时数据
	KMSG_SECURITYLIST           = 0x0450 // 证券列表
	KMSG_SECURITYQUOTES         = 0x053e // 行情信息
	KMSG_TRANSACTIONDATA        = 0x0fc5 // 分笔成交信息
	KMSG_XDXRINFO               = 0x000f // 除权除息信息

)


*/
