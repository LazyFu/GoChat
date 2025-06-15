// File: cmd/cli-tester/main.go
package main

import (
	"ChatTool/internal/client" // 复用客户端核心
	"ChatTool/pkg/protocol"    // 引入协议
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	// --- 1. 获取用户名 ---
	fmt.Print("请输入您的用户名: ")
	reader := bufio.NewReader(os.Stdin)
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	// --- 2. 连接服务器 ---
	coreClient := client.NewClient()
	if err := coreClient.Connect("127.0.0.1:8080"); err != nil {
		fmt.Println("错误：无法连接到服务器:", err)
		return
	}
	defer coreClient.Close()
	coreClient.Start()
	fmt.Printf("已成功连接到服务器，您是 %s。\n", username)

	// --- 3. 发送登录消息 ---
	loginMsg := protocol.Message{Type: protocol.LoginRequest, Sender: username}
	coreClient.Send(loginMsg)

	// --- 4. 启动一个goroutine专门监听并打印服务器消息 ---
	go listenServer(coreClient)

	// --- 5. 主goroutine负责监听用户键盘输入 ---
	listenInput(coreClient, username)
}

// listenServer 监听并打印来自服务器的所有消息
func listenServer(c *client.Client) {
	for msg := range c.GetIncomingMessages() {
		// 根据消息类型，用不同的格式打印出来
		switch msg.Type {
		case protocol.TreeUpdate:
			fmt.Println("\n--- [系统通知] 用户/群组列表已更新 ---")
			fmt.Println("在线用户:")
			for _, user := range msg.TreePayload.Users {
				fmt.Printf("- %s\n", user)
			}
			fmt.Println("可用群组:")
			for groupName, members := range msg.TreePayload.Groups {
				fmt.Printf("- %s (%d人): %v\n", groupName, len(members), members)
			}
			fmt.Println("------------------------------------")
		case protocol.BroadcastMessage, protocol.GroupMessage, protocol.PrivateMessage:
			fmt.Printf("\n[%s] %s: %s\n", msg.Sender, formatRecipient(msg), msg.TextPayload)
		default:
			fmt.Printf("\n[系统消息]: %s\n", msg.TextPayload)
		}
		fmt.Print("> ") // 打印提示符，方便继续输入
	}
	fmt.Println("与服务器的连接已断开。")
}

// listenInput 监听并处理用户的键盘输入
func listenInput(c *client.Client, username string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("--- 命令提示 ---")
	fmt.Println("/create <群名>  - 创建群组")
	fmt.Println("/join <群名>    - 加入群组")
	fmt.Println("/pm <用户名> <消息> - 发送私聊")
	fmt.Println("直接输入内容     - 发送广播消息")
	fmt.Println("----------------")

	for {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		parts := strings.SplitN(input, " ", 2)
		command := parts[0]

		var msg protocol.Message
		msg.Sender = username

		switch command {
		case "/create":
			if len(parts) > 1 {
				msg.Type = protocol.CreateGroupRequest
				msg.TextPayload = parts[1] // 使用Payload传递群名
			}
		case "/join":
			// 在我们新的Hub设计中，客户端只需要发送一个JoinGroupRequest
			// 这是一个待实现的命令
		case "/pm":
			if len(parts) > 1 {
				pmParts := strings.SplitN(parts[1], " ", 2)
				if len(pmParts) == 2 {
					msg.Type = protocol.PrivateMessage
					msg.Recipient = pmParts[0]
					msg.TextPayload = pmParts[1]
				}
			}
		default:
			msg.Type = protocol.BroadcastMessage
			msg.TextPayload = input
		}

		if msg.Type != "" {
			c.Send(msg)
		}
	}
}

func formatRecipient(msg protocol.Message) string {
	if msg.GroupName != "" {
		return fmt.Sprintf("在群组[%s]说", msg.GroupName)
	}
	if msg.Recipient != "" {
		return fmt.Sprintf("私聊对[%s]说", msg.Recipient)
	}
	return "说"
}
