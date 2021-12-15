// 2021.12.14
// gob 解码器

package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

type GobCodec struct {
	conn io.ReadWriteCloser // tcp 连接
	buf  *bufio.Writer      // 缓存
	dec  *gob.Decoder       // 解码器
	enc  *gob.Encoder       // 编码器
}

var _ Codec = (*GobCodec)(nil)

func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)

	return &GobCodec{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(buf),
	}
}

/* ------------- 安排 Codec 接口 ------------- */

// 关闭
func (c *GobCodec) Close() error {
	return c.conn.Close()
}

// 读取头部
func (c *GobCodec) ReadHeader(h *Header) error {
	return c.dec.Decode(h)
}

// 读取消息体
func (c *GobCodec) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

// 写消息 编码
func (c *GobCodec) Write(h *Header, body interface{}) (err error) {
	defer func() {
		_ = c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()

	if err := c.enc.Encode(h); err != nil {
		log.Println("ljr rpc codec gob error encoding header: ", err)
		return err
	}

	if err := c.enc.Encode(body); err != nil {
		log.Println("ljr rpc codec gob error encoding body: ", err)
		return err
	}

	return nil
}
