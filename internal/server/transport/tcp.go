package transport

import (
	"ChatTool/internal/server/core"
	"fmt"
	"net"
)

// Server 定义了 TCP 服务器结构体
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
		// 接受新的客户端连接
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("接受连接失败: %v\n", err)
			continue
		}

		// 创建新的客户端实例
		client := core.NewClient(s.hub, conn)
		// 将新客户端注册到 Hub
		s.hub.Register <- client
		// 启动两个 goroutine 处理连接
		go client.Start()
	}
}
