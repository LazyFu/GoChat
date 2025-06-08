package transport

import (
	"GoChat/internal/server/core"
	"fmt"
	"net"
)

type Server struct {
	Address string    // 监听地址
	Port    int       // 监听端口
	hub     *core.Hub // 指向中心枢纽的指针
}

func NewServer(address string, port int, hub *core.Hub) *Server {
	return &Server{
		Address: address,
		Port:    port,
		hub:     hub,
	}
}

// Start 启动 TCP 服务器
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.Address, s.Port))
	if err != nil {
		return err
	}
	defer listener.Close()

	fmt.Printf("服务器已启动，监听地址: %s:%d\n", s.Address, s.Port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("接受连接失败: %v\n", err)
			continue
		}
		client := core.NewClient(s.hub, conn)
		s.hub.Register <- client
		go client.Start()
	}
}
