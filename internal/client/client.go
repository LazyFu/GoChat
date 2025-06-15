package client

import (
	"ChatTool/pkg/protocol"
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
)

type Client struct {
	username string        // 客户端唯一标识（如用户ID或连接地址）
	conn     net.Conn      // TCP 连接
	reader   *bufio.Reader // 用于读取数据的缓冲读取器
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc    // 用于取消上下文
	incoming chan protocol.Message // 用于接收来自 Hub 的消息
	outgoing chan protocol.Message // 用于发送消息到 Hub
}

func NewClient() *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		ctx:      ctx,
		cancel:   cancel,
		incoming: make(chan protocol.Message, 256), // 带缓冲的通道
		outgoing: make(chan protocol.Message, 256), // 带缓冲的通道
	}
}

// Connect 连接到服务器
func (c *Client) Connect(address string) error {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}
	c.conn = conn
	c.reader = bufio.NewReader(conn)
	return nil
}

func (c *Client) Start() {
	fmt.Println("Client.Start(): 启动 sendLoop 和 receiveLoop") // ✅
	c.wg.Add(2)
	go c.receiveLoop()
	go c.sendLoop()
}

// receiveLoop 处理接收消息
func (c *Client) receiveLoop() {
	defer c.wg.Done()
	defer close(c.incoming)
	fmt.Println("receiveLoop 开始等待消息") // ✅
	for {
		select {
		case <-c.ctx.Done():
			fmt.Println("receiveLoop 检测到 ctx.Done，退出") // ✅
			return
		default:
			message, err := protocol.DecodeMessage(c.reader)
			if err != nil {
				fmt.Println("receiveLoop 读取失败:", err) // ✅
				if err != io.EOF || isNetClosedErr(err) {
					fmt.Println("Connection closed by server:", err)
				}
				c.Close()
				return
			}

			// 增强日志，打印更多消息细节
			if message.Type == protocol.TreeUpdate {
				fmt.Printf("TreeUpdate收到: 用户数:%d, 群组数:%d\n",
					len(message.TreePayload.Users),
					len(message.TreePayload.Groups))
			} else {
				fmt.Println("receiveLoop 收到消息:", message.Type) // ✅
			}

			c.incoming <- *message
		}
	}
}

func (c *Client) sendLoop() {
	defer c.wg.Done()
	for {
		select {
		case <-c.ctx.Done():
			return
		case message := <-c.outgoing:
			frame, err := protocol.EncodeMessage(message)
			if err != nil {
				fmt.Println("Error encoding message:", err)
				continue
			}
			if _, err := c.conn.Write(frame); err != nil {
				if isNetClosedErr(err) {
					fmt.Println("Connection closed by server:", err)
				} else {
					fmt.Println("Error writing to connection:", err)
				}
				c.Close()
				return
			}
		}
	}
}

// SetUsername 设置该客户端的用户名
func (c *Client) SetUsername(name string) {
	c.username = name
}

// SendChatMessage 是一个更高级的发送函数
func (c *Client) SendChatMessage(msgType, recipient, groupName, payload string) {
	message := protocol.Message{
		Type:        msgType,
		Sender:      c.username, // <-- 使用自己保管的username
		Recipient:   recipient,
		GroupName:   groupName,
		TextPayload: payload,
	}
	c.Send(message)
}

func (c *Client) Send(msg protocol.Message) {
	select {
	case c.outgoing <- msg:
	case <-c.ctx.Done():
		fmt.Println("Client is closed, cannot send message")
	}
}

func (c *Client) GetIncomingMessages() <-chan protocol.Message {
	return c.incoming
}

func (c *Client) Close() {
	c.cancel() // 取消上下文，通知所有 goroutine 停止
	if c.conn != nil {
		c.conn.Close() // 关闭连接
	}
	//c.wg.Wait() // 等待所有 goroutine 完成
}

func isNetClosedErr(err error) bool {
	if err == nil {
		return false
	}
	if OpErr, ok := err.(*net.OpError); ok {
		return OpErr.Err.Error() == "use of closed network connection"
	}
	return err == net.ErrClosed || err == io.EOF
}
