package core

import "sync"

type Group struct {
	Name    string           // 群组名称
	Clients map[*Client]bool // 成员列表
	mu      sync.RWMutex     // 保护 Clients 的互斥锁
}

func NewGroup(name string) *Group {
	return &Group{
		Name:    name,
		Clients: make(map[*Client]bool),
	}
}

// AddClient 将客户端添加到群组
func (g *Group) AddClient(client *Client) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Clients[client] = true
}

// RemoveClient 将客户端从群组移除
func (g *Group) RemoveClient(client *Client) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.Clients, client)
}
