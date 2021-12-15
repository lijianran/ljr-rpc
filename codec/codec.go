// 2021.12.14
// 消息序列化与反序列化

package codec

import "io"

// 头部
type Header struct {
	ServiceMethod string // 服务名.方法名
	Seq           uint64 // 请求编号
	Error         string // 错误消息
}

// 抽象解码器
type Codec interface {
	io.Closer
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Write(*Header, interface{}) error
}

// 构造函数
type NewCodecFunc func(io.ReadWriteCloser) Codec

// 解码器类型
type Type string

const (
	GobType  Type = "application/gob"
	JsonType Type = "application/json"
)

// 构造函数 map
var NewCodeFuncMap map[Type]NewCodecFunc

func init() {
	NewCodeFuncMap = make(map[Type]NewCodecFunc, 0)
	NewCodeFuncMap[GobType] = NewGobCodec
}
