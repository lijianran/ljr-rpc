// 2021.12.15
// 客户端

package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"ljr-rpc/codec"
	"ljr-rpc/server"
)

// rpc call
type Call struct {
	Seq           uint64      // 请求编码
	ServiceMethod string      // 服务名.方法名
	Args          interface{} // 方法参数
	Reply         interface{} // 方法响应
	Error         error       // 错误消息
	Done          chan *Call  // 调用完成提醒
}

// 调用结束时通知调用方
func (call *Call) done() {
	call.Done <- call
}

// rpc client
type Client struct {
	cc       codec.Codec      // 编码解码器
	opt      *server.Option   // 协议
	sending  sync.Mutex       // 锁
	header   codec.Header     // 请求头部
	mu       sync.Mutex       // 锁
	seq      uint64           // 请求编码
	pending  map[uint64]*Call // 待发送的 Call
	closing  bool             // 客户端是否关闭
	shutdown bool             // 服务器是否关闭
}

// 判断接口是否全部实现
var _ io.Closer = (*Client)(nil)

// 连接关闭错误消息
var ErrShutdown = errors.New("connection is shutdown")

// 关闭
func (client *Client) Close() error {
	// 上锁
	client.mu.Lock()
	defer client.mu.Unlock()

	if client.closing {
		return ErrShutdown
	}
	client.closing = true
	return client.cc.Close()
}

// 服务器是否运作
func (client *Client) IsAvailable() bool {
	// 上锁
	client.mu.Lock()
	defer client.mu.Unlock()

	return !client.shutdown && !client.closing
}

// 注册 Call -> client.pending
func (client *Client) registerCall(call *Call) (uint64, error) {
	// 上锁
	client.mu.Lock()
	defer client.mu.Unlock()

	if client.closing || client.shutdown {
		return 0, ErrShutdown
	}

	call.Seq = client.seq
	client.pending[call.Seq] = call
	client.seq++

	return call.Seq, nil
}

// 移除 Call <- client.pending
func (client *Client) removeCall(seq uint64) *Call {
	// 上锁
	client.mu.Lock()
	defer client.mu.Unlock()

	call := client.pending[seq]
	delete(client.pending, seq)

	return call
}

// 终止 Call
func (client *Client) terminateCalls(err error) {
	// 上锁
	client.sending.Lock()
	defer client.sending.Unlock()

	client.mu.Lock()
	defer client.mu.Unlock()

	client.shutdown = true
	for _, call := range client.pending {
		// 有错误时 将错误通知 pending 的每一个 call
		call.Error = err
		call.done()
	}
}

// 接受响应
func (client *Client) receive() {
	var err error
	for err == nil {
		var h codec.Header
		if err = client.cc.ReadHeader(&h); err != nil {
			// 读取头部失败
			break
		}

		call := client.removeCall(h.Seq)
		switch {
		case call == nil:
			err = client.cc.ReadBody(nil)

		case h.Error != "":
			call.Error = fmt.Errorf(h.Error)
			err = client.cc.ReadBody(nil)
			call.done()

		default:
			// 读取调用方法的响应
			err = client.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body " + err.Error())
			}
			call.done()

		}
	}

	// 出现错误
	client.terminateCalls(err)
}

// 客户端生成器
func NewClient(conn net.Conn, opt *server.Option) (*Client, error) {
	// 编码解码器
	f := codec.NewCodeFuncMap[opt.CodecType]
	if f == nil {
		err := fmt.Errorf("invalid codec type %s", opt.CodecType)
		log.Println("ljr rpc client codec error: ", err)
		return nil, err
	}

	// 发送 Option 给服务器
	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		log.Println("ljr rpc client options error: ", err)
		// 发生错误 关闭连接
		_ = conn.Close()
		return nil, err
	}

	return newClientCodec(f(conn), opt), nil
}

// 客户端编码解码器
func newClientCodec(cc codec.Codec, opt *server.Option) *Client {
	client := &Client{
		seq:     1,
		cc:      cc,
		opt:     opt,
		pending: make(map[uint64]*Call, 0),
	}

	// 开始接受消息响应
	go client.receive()

	return client
}

// 解析 Option
func parseOptions(opts ...*server.Option) (*server.Option, error) {
	// option 参数错误
	if len(opts) == 0 || opts[0] == nil {
		return server.DefautlOption, nil
	}

	if len(opts) != 1 {
		return nil, errors.New("number of options is more than 1")
	}

	opt := opts[0]
	opt.MagicNumber = server.DefautlOption.MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = server.DefautlOption.CodecType
	}

	return opt, nil
}

// 连接服务器
func Dial(network string, address string, opts ...*server.Option) (client *Client, err error) {
	// 解析 Option
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}

	// 连接服务器
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}

	defer func() {
		if client == nil {
			_ = conn.Close()
		}
	}()

	return NewClient(conn, opt)
}

// 发送请求
func (client *Client) send(call *Call) {
	// 上锁
	client.sending.Lock()
	defer client.sending.Unlock()

	// 注册 Call -> pending
	seq, err := client.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	// 准备请求的头部
	client.header.ServiceMethod = call.ServiceMethod
	client.header.Seq = seq
	client.header.Error = ""

	// 编码并发送请求
	if err := client.cc.Write(&client.header, call.Args); err != nil {
		// 发生错误 移除 Call <- pending
		call := client.removeCall(seq)
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

// 异步请求
func (client *Client) Go(serviceMethod string, args interface{}, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("ljr rpc client done channel is unbuffered")
	}

	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}

	client.send(call)

	return call
}

// 同步方法
func (client *Client) Call(serviceMethod string, args interface{}, reply interface{}) error {
	// 阻塞 等待 Done 返回请求响应
	call := <-client.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Error
}
