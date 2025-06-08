package core

import (
	"GoChat/pkg/protocol"
	"fmt"
	"sync"
)

type GroupCommand struct {
	Client    *Client
	GroupName string
}

type Hub struct {
	Clients    map[string]*Client
	Groups     map[string]*Group
	Register   chan *Client
	Unregister chan *Client
	JoinGroup  chan *GroupCommand
	LeaveGroup chan *GroupCommand
	Forward    chan *protocol.Message
	mu         sync.RWMutex
	groupMu    sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[string]*Client),
		Groups:     make(map[string]*Group),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		JoinGroup:  make(chan *GroupCommand),
		LeaveGroup: make(chan *GroupCommand),
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
		case cmd := <-h.JoinGroup:
			h.handleJoinGroup(cmd.Client, cmd.GroupName)
		case cmd := <-h.LeaveGroup:
			h.handleLeaveGroup(cmd.Client, cmd.GroupName)
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
	h.removeClientFromAllGroups(client)
	h.mu.Lock()
	if _, ok := h.Clients[client.ID]; ok {
		delete(h.Clients, client.ID)
		close(client.Send)
		fmt.Printf("客户端已注销: %s (Username: %s)\n", client.ID, client.Username)
	}
	h.mu.Unlock()
	h.broadcastPresence()
}

func (h *Hub) handleJoinGroup(client *Client, groupName string) {
	h.groupMu.Lock()
	group, ok := h.Groups[groupName]
	if !ok {
		group = NewGroup(groupName)
		h.Groups[groupName] = group
		fmt.Printf("新群组被自动创建: %s\n", groupName)
	}
	h.groupMu.Unlock()

	group.AddClient(client)
	fmt.Printf("客户端 %s 加入了群组 %s\n", client.Username, groupName)
	h.broadcastPresence()
}

func (h *Hub) handleLeaveGroup(client *Client, groupName string) {
	h.groupMu.Lock()
	group, ok := h.Groups[groupName]
	if ok {
		group.RemoveClient(client)
		fmt.Printf("客户端 %s 离开了群组 %s\n", client.Username, groupName)
		if len(group.Clients) == 0 {
			delete(h.Groups, groupName)
			fmt.Printf("群组 %s 因成员为空已被销毁。\n", groupName)
		}
	}
	h.groupMu.Unlock()
	h.broadcastPresence()
}

func (h *Hub) handleForwardMessage(message *protocol.Message) {
	switch message.Type {
	case protocol.GroupMessage, protocol.GroupFileMessage:
		h.sendGroupMessage(message)
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

	h.groupMu.RLock()
	groups := make(map[string][]string)
	for _, g := range h.Groups {
		members := []string{}
		g.mu.RLock()
		for c := range g.Clients {
			members = append(members, c.Username)
		}
		g.mu.RUnlock()
		groups[g.Name] = members
	}
	h.groupMu.RUnlock()

	treeData := protocol.TreePayload{Users: users, Groups: groups}
	message := protocol.Message{Type: protocol.TreeUpdate, TreePayload: treeData}

	for _, client := range allClients {
		select {
		case client.Send <- message:
		default:
			fmt.Printf("警告: 客户端 %s 的消息通道已满，状态更新消息被丢弃。\n", client.Username)
		}
	}
}

func (h *Hub) removeClientFromAllGroups(client *Client) {
	h.groupMu.RLock()
	groupsToModify := make([]*Group, 0)
	for _, group := range h.Groups {
		if _, ok := group.Clients[client]; ok {
			groupsToModify = append(groupsToModify, group)
		}
	}
	h.groupMu.RUnlock()

	for _, group := range groupsToModify {
		h.handleLeaveGroup(client, group.Name)
	}
}

func (h *Hub) sendGroupMessage(message *protocol.Message) {
	h.groupMu.RLock()
	defer h.groupMu.RUnlock()

	if group, ok := h.Groups[message.GroupName]; ok {
		group.mu.RLock()
		defer group.mu.RUnlock()

		for client := range group.Clients {
			select {
			case client.Send <- *message:
				fmt.Printf("消息已发送到群组 %s 的客户端 %s: %s\n", message.GroupName, client.Username, message.TextPayload)
			default:
				fmt.Printf("警告: 群组 %s 的客户端 %s 的消息通道已满，消息被丢弃。\n", message.GroupName, client.Username)
			}
		}
	} else {
		fmt.Printf("警告: 群组 %s 不存在，无法发送消息。\n", message.GroupName)
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
