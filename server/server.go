// 2021.12.14
// 服务端

package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"sync"

	"ljr-rpc/codec"
)

const MagicNumber = 0x3bef5c

type Option struct {
	MagicNumber int
	CodecType   codec.Type
}

/*
| Option{MagicNumber: xxx, CodecType: xxx} | Header{ServiceMethod ...} | Body interface{} |
| <------      固定 JSON 编码      ------>  | <-------   编码方式由 CodeType 决定   ------->|

| Option | Header1 | Body1 | Header2 | Body2 | ...
*/
var DefautlOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,
}

// 服务器
type Server struct{}

// 构造函数
func NewServer() *Server {
	return &Server{}
}

// 默认的服务器实例
var DefaultServer = NewServer()

func (server *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("ljr rpc server accept connection error: ", err)
			return
		}

		go server.ServeConn(conn)
	}
}

func Accept(lis net.Listener) {
	DefaultServer.Accept(lis)
}

// 服务协程
func (server *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() {
		_ = conn.Close()
	}()

	var opt Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("ljr rpc server options error: ", err)
		return
	}
	if opt.MagicNumber != MagicNumber {
		log.Printf("ljr rpc server recv invalid magic number %x", opt.MagicNumber)
		return
	}

	f := codec.NewCodeFuncMap[opt.CodecType]
	if f == nil {
		log.Printf("ljr rpc server recv invalid codec type %s", opt.CodecType)
		return
	}

	// 解码器
	server.serveCodec(f(conn))
}

var invalidRequest = struct{}{}

// 处理请求
func (server *Server) serveCodec(cc codec.Codec) {
	sending := new(sync.Mutex)
	wg := new(sync.WaitGroup)

	for {
		// 读取请求
		req, err := server.readRequest(cc)
		if err != nil {
			// 有错误
			if req == nil {
				// 头部解析错误 直接返回
				break
			}

			// 将错误存入该请求头部中 request.header.Error
			req.h.Error = err.Error()
			// 回复请求
			server.sendResponse(cc, req.h, invalidRequest, sending)
			continue
		}

		wg.Add(1)
		// 处理请求
		go server.handleRequest(cc, req, sending, wg)
	}

	wg.Wait()
	_ = cc.Close()
}

// 请求体
type request struct {
	h      *codec.Header
	argv   reflect.Value
	replyv reflect.Value
}

// 读取头部 request.header
func (server *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != nil {
			log.Println("ljr rpc server read header error: ", err)
		}
		return nil, err
	}

	return &h, nil
}

// 读取请求 request
func (server *Server) readRequest(cc codec.Codec) (*request, error) {
	// 读取头部 request.header
	h, err := server.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}

	req := &request{h: h}

	// 默认参数为 string
	req.argv = reflect.New(reflect.TypeOf(""))

	// 读取请求体 request.body
	if err = cc.ReadBody(req.argv.Interface()); err != nil {
		log.Println("ljr rpc server read argv error: ", err)
	}

	return req, nil
}

// 回复请求
func (server *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	// 上锁
	sending.Lock()
	defer sending.Unlock()

	if err := cc.Write(h, body); err != nil {
		log.Println("ljr rpc server write response error: ", err)
	}
}

// 处理请求
func (server *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()

	log.Println(req.h, req.argv.Elem())
	req.replyv = reflect.ValueOf(fmt.Sprintf("ljr rpc response: %d", req.h.Seq))
	server.sendResponse(cc, req.h, req.replyv.Interface(), sending)
}
