package main

import (
	"ChatTool/internal/client"
	"ChatTool/pkg/protocol"
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2/app"
)

// 定义服务器的地址和端口
const serverAddress = "127.0.0.1:8080"

func main() {
	fyneApp := app.New()

	var username string
	if len(os.Args) > 1 {
		username = os.Args[1]
	} else {
		username = fmt.Sprintf("User_%d", time.Now().UnixNano()%1000)
		fmt.Printf("未提供用户名，将使用默认名称: %s\n", username)
		fmt.Println("用法: go run ./cmd/client <你的用户名>")
	}

	coreClient := client.NewClient()

	fmt.Printf("[%s] 正在连接到服务器 %s...\n", username, serverAddress)
	if err := coreClient.Connect(serverAddress); err != nil {
		fmt.Printf("错误：无法连接到服务器: %v\n", err)
		fmt.Println("请确保服务器程序已经在一个独立的终端中运行。")
		os.Exit(1) // 连接失败，程序退出
	}
	fmt.Printf("[%s] 成功连接到服务器！\n", username)

	coreClient.Start()

	loginMessage := protocol.Message{
		Type:   protocol.LoginRequest, // 使用一个特殊的类型来标识这是登录/注册消息
		Sender: username,
	}
	coreClient.Send(loginMessage) // 将消息放入发送队列

	gui := client.NewUI(fyneApp, coreClient, username)

	gui.Run()
	coreClient.Close() // 确保在程序结束时关闭客户端连接
	fmt.Printf("[%s] 客户端已关闭。\n", username)
}
