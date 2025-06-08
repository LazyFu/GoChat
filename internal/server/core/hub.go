package core

import (
	"ChatTool/pkg/protocol"
	"encoding/json"
	"fmt"
	"sync"
)

// Hub 负责管理所有客户端连接和消息广播
type Hub struct {
	Clients    map[string]*Client     // 存储所有已连接的客户端
	Register   chan *Client           // 注册新客户端的通道
	Unregister chan *Client           // 注销客户端的通道
	Forward    chan *protocol.Message // 用于转发消息的通道
	mu         sync.Mutex             // 保护 clients 的互斥锁
}

// NewHub 创建一个新的 Hub 实例
func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[string]*Client),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Forward:    make(chan *protocol.Message),
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

		case message := <-h.Forward:
			// 处理消息转发
			h.mu.Lock()
			switch message.Type {
			case protocol.PrivateMessage:
				// 私聊消息处理
				if recipient, ok := h.Clients[message.Recipient]; ok {
					payload, _ := json.Marshal(message)
					recipient.Send <- payload
				}
			case protocol.GroupMessage:
				// 群聊消息处理
				for _, client := range h.Clients {
					payload, _ := json.Marshal(message)
					client.Send <- payload
				}
			default:
				// 广播消息处理
				for _, client := range h.Clients {
					payload, _ := json.Marshal(message)
					client.Send <- payload
				}
			}
			h.mu.Unlock()
		}
	}
}
