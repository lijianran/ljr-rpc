// 2021.12.15
// ljr rpc

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"ljr-rpc/codec"
	ljrpc "ljr-rpc/server"
)

// 开启服务器
func startServer(addr chan string) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("network error: ", err)
		return
	}

	log.Println("start ljr rpc server on", l.Addr())

	addr <- l.Addr().String()
	ljrpc.Accept(l)
}

func main() {
	addr := make(chan string)
	go startServer(addr)

	// 连接 ljr rpc 服务器
	conn, _ := net.Dial("tcp", <-addr)
	defer func() {
		_ = conn.Close()
	}()

	time.Sleep(time.Second)

	// 编码 Option 协议交换
	_ = json.NewEncoder(conn).Encode(ljrpc.DefautlOption)
	cc := codec.NewGobCodec(conn)

	for i := 0; i < 5; i++ {
		// 头部
		h := &codec.Header{
			ServiceMethod: "Foo.Sum",
			Seq:           uint64(i),
		}
		// 编码 header body
		_ = cc.Write(h, fmt.Sprintf("ljr rpc req %d", h.Seq))

		// 解码读取
		_ = cc.ReadHeader(h)
		var reply string
		_ = cc.ReadBody(&reply)
		log.Println("reply: ", reply)
	}
}
