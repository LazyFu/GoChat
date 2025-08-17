package core

import (
	"GoChat/pkg/protocol"
	"bufio"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
)

// Client 表示一个连接的客户端
type Client struct {
	ID       string                // 客户端唯一标识
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

// ReadPump 负责从客户端读取数据并智能地分发到Hub的不同通道
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister <- c
		c.conn.Close()
	}()

	reader := bufio.NewReader(c.conn)
	isRegistered := false

	for {
		c.conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		message, err := protocol.DecodeMessage(reader)
		if err != nil {
			fmt.Printf("读取客户端 %s (%s) 数据失败: %v\n", c.Username, c.ID, err)
			break
		}

		message.Timestamp = time.Now()

		if c.Username != "" {
			message.Sender = c.Username
		}

		switch message.Type {

		case protocol.LoginRequest:
			if !isRegistered && message.Sender != "" {
				fmt.Println("Login")
				c.Username = message.Sender
				c.hub.Register <- c
				isRegistered = true
			}

		case protocol.CreateGroupRequest:
			fmt.Println("CreateGroup")
			cmd := &GroupCommand{
				Client:    c,
				GroupName: message.TextPayload,
			}
			c.hub.JoinGroup <- cmd

		case protocol.JoinGroupRequest:
			cmd := &GroupCommand{
				Client:    c,
				GroupName: message.GroupName,
			}
			c.hub.JoinGroup <- cmd

		case protocol.LeaveGroupRequest:
			cmd := &GroupCommand{
				Client:    c,
				GroupName: message.GroupName,
			}
			c.hub.LeaveGroup <- cmd

		case protocol.BroadcastMessage, protocol.PrivateMessage, protocol.GroupMessage,
			protocol.PrivateFileMessage, protocol.GroupFileMessage:
			if isRegistered {
				c.hub.Forward <- message
			} else {
				fmt.Printf("警告: 客户端 %s 在未登录时尝试发送聊天消息。\n", c.ID)
			}

		default:
			fmt.Printf("警告: 收到未知的消息类型: '%s'\n", message.Type)
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
