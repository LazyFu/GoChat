package core

import (
	"ChatTool/pkg/protocol"
	"fmt"
	"sync"
)

// Hub 负责管理所有客户端连接和消息广播
type Hub struct {
	Clients    map[string]*Client     // 存储所有已连接的客户端
	Register   chan *Client           // 注册新客户端的通道
	Unregister chan *Client           // 注销客户端的通道
	Forward    chan *protocol.Message // 用于转发消息的通道
	mu         sync.RWMutex           // 保护 clients 的互斥锁
}

// NewHub 创建一个新的 Hub 实例
func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[string]*Client),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Forward:    make(chan *protocol.Message),
	}
}

// Run 启动 Hub，处理注册、注销和广播事件
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			// 处理客户端注册
			h.mu.Lock()
			h.Clients[client.ID] = client
			h.mu.Unlock()
			fmt.Printf("客户端已注册: %s\n", client.ID)

		case client := <-h.Unregister:
			// 处理客户端注销
			h.mu.Lock()
			if _, ok := h.Clients[client.ID]; ok {
				delete(h.Clients, client.ID)
				close(client.Send)
				fmt.Printf("客户端已注销: %s\n", client.ID)
			}
			h.mu.Unlock()

		case message := <-h.Forward:
			// 处理消息转发
			h.broadcast(message)
			// 	h.mu.Lock()
			// 	switch message.Type {
			// 	case protocol.PrivateMessage:
			// 		// 私聊消息处理
			// 		if recipient, ok := h.Clients[message.Recipient]; ok {
			// 			recipient.Send <- *message
			// 		}
			// 	case protocol.GroupMessage:
			// 		// 群聊消息处理
			// 		for _, client := range h.Clients {
			// 			client.Send <- *message
			// 		}
			// 	default:
			// 		// 广播消息处理
			// 		for _, client := range h.Clients {
			// 			client.Send <- *message
			// 		}
			// 	}
			// 	h.mu.Unlock()
		}
	}
}

func (h *Hub) broadcast(message *protocol.Message) {
	// 变化点 3：只在读取客户端列表时加锁，并尽快释放
	h.mu.RLock()         // 读操作，上读锁
	defer h.mu.RUnlock() // 函数结束时自动释放读锁

	// (您的私聊和群聊逻辑可以放在这里)
	// 这里我们先实现一个健壮的广播
	for _, client := range h.Clients {
		// 变化点 4：使用非阻塞发送，防止单个慢客户端冻结整个hub
		select {
		case client.Send <- *message:
			// 消息成功放入通道
		default:
			// 如果接收方的Send通道满了，说明该客户端处理不过来。
			// 我们不能等待，而是直接跳过，并考虑将其断开。
			fmt.Printf("警告: 客户端 %s 的消息通道已满，消息被丢弃。\n", client.ID)
			// 在实际生产中，您可能会在这里触发一个清理机制，
			// 但现在简单地丢弃消息可以防止死锁。
		}
	}
}
