package main

import (
	"fmt"
	"log"

	"ChatTool/internal/server/core"
	"ChatTool/internal/server/transport"
)

func main() {
	// 初始化 Hub
	hub := core.NewHub()
	go hub.Run() // 启动 Hub 的事件处理循环

	// 创建 TCP 服务器
	server := &transport.Server{
		Address: "0.0.0.0", // 监听所有网络接口
		Port:    8080,      // 监听端口
	}

	fmt.Println("服务器正在启动...")
	err := server.Start()
	if err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
