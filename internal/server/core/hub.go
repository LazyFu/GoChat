package core

import (
	"GoChat/pkg/protocol"
	"fmt"
	"sync"
)

type Hub struct {
	Clients    map[string]*Client
	Register   chan *Client
	Unregister chan *Client
	Forward    chan *protocol.Message
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[string]*Client),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Forward:    make(chan *protocol.Message),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.handleRegister(client)
		case client := <-h.Unregister:
			h.handleUnregister(client)
		case message := <-h.Forward:
			h.handleForwardMessage(message)
		}
	}
}

func (h *Hub) handleRegister(client *Client) {
	h.mu.Lock()
	h.Clients[client.ID] = client
	h.mu.Unlock()
	fmt.Printf("客户端已注册: %s (Username: %s)\n", client.ID, client.Username)
	h.broadcastPresence()
}

func (h *Hub) handleUnregister(client *Client) {
	h.mu.Lock()
	if _, ok := h.Clients[client.ID]; ok {
		delete(h.Clients, client.ID)
		close(client.Send)
		fmt.Printf("客户端已注销: %s (Username: %s)\n", client.ID, client.Username)
	}
	h.mu.Unlock()
	h.broadcastPresence()
}

func (h *Hub) handleForwardMessage(message *protocol.Message) {
	switch message.Type {
	case protocol.PrivateMessage, protocol.PrivateFileMessage:
		h.sendPrivateMessage(message)
	case protocol.BroadcastMessage:
		h.broadcastMessage(message)
	}
}

// broadcastPresence 是统一的、唯一的“状态广播”函数
func (h *Hub) broadcastPresence() {
	h.mu.RLock()
	allClients := make([]*Client, 0, len(h.Clients))
	users := make([]string, 0, len(h.Clients))
	for _, client := range h.Clients {
		allClients = append(allClients, client)
		users = append(users, client.Username)
	}
	h.mu.RUnlock()

	treeData := protocol.TreePayload{Users: users}
	message := protocol.Message{Type: protocol.TreeUpdate, TreePayload: treeData}

	for _, client := range allClients {
		select {
		case client.Send <- message:
		default:
			fmt.Printf("警告: 客户端 %s 的消息通道已满，状态更新消息被丢弃。\n", client.Username)
		}
	}
}

func (h *Hub) sendPrivateMessage(message *protocol.Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if recipient, ok := h.findClientByUsername(message.Recipient); ok {
		select {
		case recipient.Send <- *message:
		default:
			fmt.Printf("私聊接收方 %s 的消息通道已满。\n", recipient.Username)
		}
	}
	if sender, ok := h.findClientByUsername(message.Sender); ok {
		select {
		case sender.Send <- *message:
		default:
			fmt.Printf("私聊发送方 %s 的消息通道已满。\n", sender.Username)
		}
	}
}

func (h *Hub) findClientByUsername(username string) (*Client, bool) {
	for _, client := range h.Clients {
		if client.Username == username {
			return client, true
		}
	}
	return nil, false
}

func (h *Hub) broadcastMessage(message *protocol.Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, client := range h.Clients {
		select {
		case client.Send <- *message:
			fmt.Printf("消息已发送到客户端 %s: %s\n", client.ID, message.TextPayload)
		default:
			fmt.Printf("警告: 客户端 %s 的消息通道已满，消息被丢弃。\n", client.ID)
		}
	}
}
