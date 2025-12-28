package client

import (
	chatcrypto "GoChat/internal/crypto"
	"GoChat/pkg/protocol"
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
)

const defaultKey = "1234567890abcdefghijklmnopqrstuv" // 32 bytes plain text for AES-256 demo

type Client struct {
	username          string        // 客户端唯一标识j
	conn              net.Conn      // TCP 连接
	reader            *bufio.Reader // 用于读取数据的缓冲读取器
	wg                sync.WaitGroup
	ctx               context.Context
	cancel            context.CancelFunc    // 用于取消上下文
	incoming          chan protocol.Message // 用于接收来自 Hub 的消息
	outgoing          chan protocol.Message // 用于发送消息到 Hub
	secretKey         []byte
	encryptionEnabled bool
}

func NewClient() *Client {
	ctx, cancel := context.WithCancel(context.Background())
	key, enabled := loadSymmetricKeyFromEnv()
	return &Client{
		ctx:               ctx,
		cancel:            cancel,
		incoming:          make(chan protocol.Message, 256), // 带缓冲的通道
		outgoing:          make(chan protocol.Message, 256), // 带缓冲的通道
		secretKey:         key,
		encryptionEnabled: enabled,
	}
}

func loadSymmetricKeyFromEnv() ([]byte, bool) {
	keyStr := os.Getenv("GOCHAT_PSK")
	if keyStr == "" {
		return []byte(defaultKey), true
	}
	if len(keyStr) == 32 {
		return []byte(keyStr), true
	}
	if decoded, err := base64.StdEncoding.DecodeString(keyStr); err == nil && len(decoded) == 32 {
		return decoded, true
	}
	fmt.Printf("GOCHAT_PSK 无效（原始长度=%d，Base64解码失败或非32字节），回退到演示密钥\n", len(keyStr))
	return []byte(defaultKey), true
}

func isValidKeyLength(l int) bool {
	return l == 16 || l == 24 || l == 32
}

// SetSecretKey allows overriding the symmetric key at runtime.
func (c *Client) SetSecretKey(key []byte) error {
	if !isValidKeyLength(len(key)) {
		return errors.New("密钥长度必须为 16/24/32 字节")
	}
	c.secretKey = key
	c.encryptionEnabled = true
	return nil
}

func (c *Client) encryptTextMessage(message *protocol.Message) error {
	if !c.encryptionEnabled {
		return nil
	}
	ad := buildAssociatedData(message)
	nonce, ciphertext, err := chatcrypto.EncryptAESGCM([]byte(message.TextPayload), c.secretKey, ad)
	if err != nil {
		return err
	}
	message.Nonce = nonce
	message.Encrypted = true
	message.TextPayload = base64.StdEncoding.EncodeToString(ciphertext)
	return nil
}

func (c *Client) encryptFileMessage(message *protocol.Message) error {
	if !c.encryptionEnabled {
		return nil
	}
	ad := buildAssociatedData(message)
	nonce, ciphertext, err := chatcrypto.EncryptAESGCM(message.FilePayload.Data, c.secretKey, ad)
	if err != nil {
		return err
	}
	message.Nonce = nonce
	message.Encrypted = true
	message.FilePayload.Data = ciphertext
	return nil
}

func (c *Client) decryptMessage(message *protocol.Message) error {
	if !message.Encrypted {
		return nil
	}
	if !c.encryptionEnabled {
		return errors.New("收到加密消息但本地未开启加密")
	}
	if len(message.Nonce) == 0 {
		return errors.New("加密消息缺少 nonce")
	}
	ad := buildAssociatedData(message)
	switch message.Type {
	case protocol.PrivateMessage:
		ciphertext, err := base64.StdEncoding.DecodeString(message.TextPayload)
		if err != nil {
			return err
		}
		plaintext, err := chatcrypto.DecryptAESGCM(message.Nonce, ciphertext, c.secretKey, ad)
		if err != nil {
			return err
		}
		message.TextPayload = string(plaintext)
	case protocol.PrivateFileMessage:
		plaintext, err := chatcrypto.DecryptAESGCM(message.Nonce, message.FilePayload.Data, c.secretKey, ad)
		if err != nil {
			return err
		}
		message.FilePayload.Data = plaintext
	default:
		return nil
	}
	message.Encrypted = false
	return nil
}

func buildAssociatedData(message *protocol.Message) []byte {
	return fmt.Appendf(nil, "%s|%s|%s", message.Type, message.Sender, message.Recipient)
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
	fmt.Println("Client.Start(): 启动 sendLoop 和 receiveLoop")
	c.wg.Add(2)
	go c.receiveLoop()
	go c.sendLoop()
}

// receiveLoop 处理接收消息
func (c *Client) receiveLoop() {
	defer c.wg.Done()
	defer close(c.incoming)
	fmt.Println("receiveLoop 开始等待消息")
	for {
		select {
		case <-c.ctx.Done():
			fmt.Println("receiveLoop 检测到 ctx.Done，退出")
			return
		default:
			message, err := protocol.DecodeMessage(c.reader)
			if err != nil {
				fmt.Println("receiveLoop 读取失败:", err)
				if err != io.EOF || isNetClosedErr(err) {
					fmt.Println("Connection closed by server:", err)
				}
				c.Close()
				return
			}

			if message.Encrypted {
				if err := c.decryptMessage(message); err != nil {
					fmt.Println("receiveLoop 解密失败:", err)
					continue
				}
			}

			if message.Type == protocol.TreeUpdate {
				fmt.Printf("TreeUpdate收到: 用户数:%d\n",
					len(message.TreePayload.Users))
			} else {
				fmt.Println("receiveLoop 收到消息:", message.Type)
			}

			c.incoming <- *message
		}
	}
}

// sendLoop 处理发送消息
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

func (c *Client) SetUsername(name string) {
	c.username = name
}

// SendChatMessage 是一个更高级的发送函数
func (c *Client) SendChatMessage(msgType, recipient, payload string) {
	message := protocol.Message{
		Type:        msgType,
		Sender:      c.username,
		Recipient:   recipient,
		TextPayload: payload,
	}
	if msgType == protocol.PrivateMessage {
		if err := c.encryptTextMessage(&message); err != nil {
			fmt.Println("私聊消息加密失败:", err)
			return
		}
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
	c.cancel()
	if c.conn != nil {
		c.conn.Close()
	}
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

// SendFile 是一个专门处理文件发送逻辑的新方法
func (c *Client) SendFile(msgType, recipient, filePath string) {
	// 将文件读取和编码等耗时操作放入后台goroutine，防止阻塞UI
	go func() {
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("错误：读取文件失败: %v\n", err)
			// 可以在这里通过channel等方式通知UI发送失败
			return
		}

		fileInfo, _ := os.Stat(filePath) // 获取文件名等信息
		encodedData := base64.StdEncoding.EncodeToString(fileData)

		fileMsg := protocol.Message{
			Type:      msgType,
			Sender:    c.username,
			Recipient: recipient,
			FilePayload: protocol.FilePayload{
				Name: fileInfo.Name(),
				Size: fileInfo.Size(),
				Data: []byte(encodedData),
			},
		}
		if msgType == protocol.PrivateFileMessage {
			if err := c.encryptFileMessage(&fileMsg); err != nil {
				fmt.Printf("错误：文件加密失败: %v\n", err)
				return
			}
		}
		c.Send(fileMsg)
	}()
}

// SaveFile 是一个处理文件保存
func (c *Client) SaveFile(fileInfo protocol.FilePayload, savePath string) {
	go func() {
		decodedData, err := base64.StdEncoding.DecodeString(string(fileInfo.Data))
		if err != nil {
			fmt.Printf("错误：Base64解码失败: %v\n", err)
			return
		}

		err = os.WriteFile(savePath, decodedData, 0644)
		if err != nil {
			fmt.Printf("错误：写入文件失败: %v\n", err)
		} else {
			fmt.Printf("文件 %s 已成功保存到 %s\n", fileInfo.Name, savePath)
		}
	}()
}
