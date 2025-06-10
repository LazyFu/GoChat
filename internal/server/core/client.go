package core

import (
	"ChatTool/pkg/protocol"
	"bufio"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
)

// Client 表示一个连接的客户端
type Client struct {
	ID       string                // 客户端唯一标识（如用户ID或连接地址）
	Username string                // 客户端用户名
	Send     chan protocol.Message // 用于向客户端发送消息的通道
	hub      *Hub                  // 指向中心枢纽的指针
	conn     net.Conn              // TCP 连接
}

// NewClient 创建一个新的 Client 实例
func NewClient(hub *Hub, conn net.Conn) *Client {
	return &Client{
		ID:   uuid.New().String(), // 生成唯一ID
		hub:  hub,
		conn: conn,
		Send: make(chan protocol.Message, 256), // 带缓冲的通道
	}
}

// ReadPump 负责从客户端读取数据并交给 Hub 处理
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister <- c // 客户端断开时通知 Hub 注销
		c.conn.Close()
	}()

	reader := bufio.NewReader(c.conn)
	for {
		// 设置读取超时
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// 读取数据
		message, err := protocol.DecodeMessage(reader)
		if err != nil {
			fmt.Printf("读取客户端数据失败: %v\n", err)
			break
		}

		message.Timestamp = time.Now()

		if c.Username == "" {
			if message.Type == protocol.LoginRequest {
				c.Username = message.Sender // 设置用户名为发送者
				c.hub.Register <- c         // 注册客户端到 Hub
			}
		} else {
			message.Sender = c.Username // 如果已设置用户名，则使用它
			c.hub.Forward <- message    // 将消息转发给 Hub
		}
	}
}

// WritePump 负责将 Hub 的消息发送给客户端
func (c *Client) WritePump() {
	defer func() {
		c.conn.Close()
	}()

	for message := range c.Send {
		// 设置写入超时
		c.conn.SetWriteDeadline(time.Now().Add(60 * time.Second))

		// 发送消息
		frame, err := protocol.EncodeMessage(message)
		if err != nil {
			fmt.Printf("编码消息失败: %v\n", err)
			continue
		}
		_, err = c.conn.Write(frame)
		if err != nil {
			fmt.Printf("发送消息失败: %v\n", err)
			return
		}
	}
	fmt.Println("发送通道已关闭，断开客户端连接")
}

// Start 启动客户端的读写协程
func (c *Client) Start() {
	go c.ReadPump()
	go c.WritePump()
}
