package main

import (
	"ChatTool/internal/server/core"
	"ChatTool/internal/server/transport"
	"fmt"
	"log"
)

func main() {
	// 初始化 Hub
	hub := core.NewHub()
	go hub.Run() // 启动 Hub 的事件处理循环

	// 创建 TCP 服务器
	server := transport.NewServer("0.0.0.0", 8080, hub)

	fmt.Println("服务器正在启动...")
	err := server.Start()
	if err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
