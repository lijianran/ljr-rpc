// 2021.12.15
// ljr rpc

package main

import (
	"log"
	"net"
	"sync"
	"time"

	"ljr-rpc/client"
	ljrpc "ljr-rpc/server"
)

type Foo int
type Args struct {
	Num1 int
	Num2 int
}

// 导出方法
func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

// 开启服务器
func startServer(addr chan string) {
	var foo Foo
	if err := ljrpc.DefaultServer.Register(&foo); err != nil {
		log.Fatal("register service error: ", err)
	}

	// 随机起一个服务端口
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("network error: ", err)
		return
	}

	log.Println("start ljr rpc server on, listening...", l.Addr())

	addr <- l.Addr().String()
	ljrpc.Accept(l)
}

func main() {
	log.SetFlags(0)

	addr := make(chan string)
	go startServer(addr)

	// 连接 ljr rpc 服务器
	// conn, _ := net.Dial("tcp", <-addr)
	rpc_client, _ := client.Dial("tcp", <-addr)
	defer func() {
		// _ = conn.Close()
		_ = rpc_client.Close()
	}()

	time.Sleep(time.Second)

	// Day 1
	// // 编码 Option 协议交换
	// _ = json.NewEncoder(conn).Encode(ljrpc.DefautlOption)
	// cc := codec.NewGobCodec(conn)

	// for i := 0; i < 5; i++ {
	// 	// 头部
	// 	h := &codec.Header{
	// 		ServiceMethod: "Foo.Sum",
	// 		Seq:           uint64(i),
	// 	}
	// 	// 编码 header body
	// 	_ = cc.Write(h, fmt.Sprintf("ljr rpc req %d", h.Seq))

	// 	// 解码读取
	// 	_ = cc.ReadHeader(h)
	// 	var reply string
	// 	_ = cc.ReadBody(&reply)
	// 	log.Println("reply: ", reply)
	// }

	// Day 2
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// args := fmt.Sprintf("ljr rpc request %d", i)
			// var reply string

			args := &Args{
				Num1: i,
				Num2: i * i,
			}
			var reply int

			if err := rpc_client.Call("Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error: ", err)
			}

			// log.Println("ljr rpc server reply: ", reply)
			log.Printf("%d + %d = %d", args.Num1, args.Num2, reply)
		}(i)
	}

	wg.Wait()
}
