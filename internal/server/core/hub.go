package core

import (
	"ChatTool/pkg/protocol"
	"fmt"
	"sync"
)

// Hub 负责管理所有客户端连接和消息广播
type Hub struct {
	Clients map[string]*Client // 存储所有已连接的客户端
	Groups  map[string]*Group  // 存储所有群组

	Register   chan *Client           // 注册新客户端的通道
	Unregister chan *Client           // 注销客户端的通道
	JoinGroup  chan *GroupCommand     // 加入群组的通道
	LeaveGroup chan *GroupCommand     // 离开群组的通道
	Forward    chan *protocol.Message // 用于转发消息的通道

	mu      sync.RWMutex // 保护 clients 的锁
	groupMu sync.RWMutex // 保护 groups 的锁
}

type GroupCommand struct {
	GroupName string  // 群组名称
	Client    *Client // 发起命令的客户端
}

// NewHub 创建一个新的 Hub 实例
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

// Run 启动 Hub，处理注册、注销和广播事件
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
	fmt.Printf("客户端已注册: %s\n", client.ID)
	h.broadcastUserList()
}

func (h *Hub) handleUnregister(client *Client) {
	h.mu.Lock()
	if _, ok := h.Clients[client.ID]; ok {
		// 用户下线前，让他离开所有已加入的群组
		for _, group := range h.Groups {
			h.handleLeaveGroup(client, group.Name)
		}
		delete(h.Clients, client.ID)
		close(client.Send)
		fmt.Printf("客户端已注销: %s (Username: %s)\n", client.ID, client.Username)
		h.broadcastUserList()
	}
	h.mu.Unlock()
}

func (h *Hub) handleJoinGroup(client *Client, groupName string) {
	// 为保证线程安全，对Groups map的写操作需要加锁
	h.groupMu.Lock()
	// 步骤1: 查找群组，如果不存在则自动创建 TODO
	group, ok := h.Groups[groupName]
	if !ok {
		group = NewGroup(groupName) // NewGroup是您之前创建的那个函数
		h.Groups[groupName] = group
		fmt.Printf("新群组被自动创建: %s\n", groupName)
		defer h.broadcastUserList()
	}
	h.groupMu.Unlock()
	group.AddClient(client)
	fmt.Printf("客户端 %s 加入了群组 %s\n", client.Username, groupName)
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
			defer h.broadcastUserAndGroupList()
		}
	}
	h.groupMu.Unlock()
}

func (h *Hub) handleForwardMessage(message *protocol.Message) {
	// 这个函数现在只关心消息应该发给谁
	if message.GroupName != "" {
		h.sendGroupMessage(message)
	} else if message.Recipient != "" {
		h.sendPrivateMessage(message)
	} else {
		h.broadcastMessage(message)
	}
}

func (h *Hub) broadcastUserAndGroupList() {
	// 调用辅助函数获取用户列表
	users := h.getUsernames()

	// --- 获取群组列表的逻辑保持不变 ---
	h.groupMu.RLock()
	defer h.groupMu.RUnlock()

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

	treeData := protocol.TreeData{Users: users, Groups: groups}
	message := protocol.Message{Type: protocol.TreeUpdate, TreePayload: treeData}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, client := range h.Clients {
		select {
		case client.Send <- message:
		default:
		}
	}
}

func (h *Hub) broadcastUserList() {
	usernames := h.getUsernames()
	message := protocol.Message{
		Type:     protocol.UserListUpdate,
		UserList: usernames,
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, client := range h.Clients {
		select {
		case client.Send <- message:
		default:
			fmt.Printf("警告: 客户端 %s 的消息通道已满，用户列表消息被丢弃。\n", client.ID)
		}
	}
}

func (h *Hub) sendGroupMessage(message *protocol.Message) {
	h.groupMu.RLock()
	defer h.groupMu.RUnlock()

	// 找到群组
	if group, ok := h.Groups[message.GroupName]; ok {
		group.mu.RLock() // 确保对群组成员的读取是线程安全的
		defer group.mu.RUnlock()

		for client := range group.Clients {
			select {
			case client.Send <- *message:
				fmt.Printf("消息已发送到群组 %s 的客户端 %s: %s\n", message.GroupName, client.Username, message.Payload)
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

	// 找到接收方
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
			fmt.Printf("消息已发送到客户端 %s: %s\n", client.ID, message.Payload)
			// 消息成功放入通道
		default:
			fmt.Printf("警告: 客户端 %s 的消息通道已满，消息被丢弃。\n", client.ID)
			// 在实际生产中，您可能会在这里触发一个清理机制，TODO
			// 但现在简单地丢弃消息可以防止死锁。
		}
	}
}

func (h *Hub) getUsernames() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	usernames := make([]string, 0, len(h.Clients))
	for _, client := range h.Clients {
		usernames = append(usernames, client.Username)
	}
	return usernames
}
