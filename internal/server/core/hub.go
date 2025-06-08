package core

import (
	"fmt"
	"sync"
)

// Hub 负责管理所有客户端连接和消息广播
type Hub struct {
	Clients    map[string]*Client // 存储所有已连接的客户端
	Register   chan *Client       // 注册新客户端的通道
	Unregister chan *Client       // 注销客户端的通道
	Broadcast  chan []byte        // 广播消息的通道
	mu         sync.Mutex         // 保护 clients 的互斥锁
}

// NewHub 创建一个新的 Hub 实例
func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[string]*Client),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast:  make(chan []byte),
		mu:         sync.Mutex{},
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

		case message := <-h.Broadcast:
			// 处理消息广播
			h.mu.Lock()
			for id, client := range h.Clients {
				select {
				case client.Send <- message:
					fmt.Printf("消息已发送给客户端: %s\n", id)
				default:
					// 如果客户端的发送通道已满，则关闭连接
					close(client.Send)
					delete(h.Clients, id)
					fmt.Printf("客户端连接已关闭: %s\n", id)
				}
			}
			h.mu.Unlock()
		}
	}
}
