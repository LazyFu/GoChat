// File: internal/client/ui.go
package client

import (
	"ChatTool/pkg/protocol" // <-- 替换成你自己的go.mod模块名
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

// UI 结构体封装了所有GUI组件
type UI struct {
	client *Client // 对核心逻辑的引用
	app    fyne.App
	window fyne.Window

	chatHistory binding.StringList // 使用数据绑定来动态、线程安全地更新聊天列表
	chatList    *widget.List
	input       *widget.Entry
	sendButton  *widget.Button
	// 可以增加一个用于显示用户名的Label
	username string
}

// NewUI 创建一个新的UI实例
func NewUI(app fyne.App, c *Client, username string) *UI {
	w := app.NewWindow("Go Fyne Chat")
	ui := &UI{
		client:      c,
		app:         app,
		window:      w,
		chatHistory: binding.NewStringList(),
		username:    username,
	}
	ui.setupUI()
	return ui
}

// setupUI 初始化并布局所有UI组件
func (ui *UI) setupUI() {
	// 创建聊天记录列表，它绑定到chatHistory数据
	ui.chatList = widget.NewListWithData(
		ui.chatHistory,
		func() fyne.CanvasObject {
			// 这是列表项的模板
			return widget.NewLabel("template")
		},
		func(i binding.DataItem, o fyne.CanvasObject) {
			// 这是如何将数据绑定到模板上
			o.(*widget.Label).Bind(i.(binding.String))
		},
	)

	// 创建输入框
	ui.input = widget.NewEntry()
	ui.input.SetPlaceHolder("Enter message...")
	// 设置当在输入框中按回车键时的回调函数
	ui.input.OnSubmitted = func(text string) {
		ui.sendMessage()
	}

	// 创建发送按钮
	ui.sendButton = widget.NewButton("Send", ui.sendMessage)

	// 将输入框和发送按钮放在一个水平容器中
	bottomBar := container.NewBorder(nil, nil, nil, ui.sendButton, ui.input)

	// 设置主内容区布局
	content := container.NewBorder(nil, bottomBar, nil, nil, ui.chatList)

	ui.window.SetContent(content)
	ui.window.Resize(fyne.NewSize(600, 400))

	// 设置窗口关闭时的回调，确保客户端核心逻辑也随之关闭
	ui.window.SetOnClosed(func() {
		ui.client.Close()
	})
}

// sendMessage 是一个辅助函数，用于处理发送逻辑
func (ui *UI) sendMessage() {
	text := ui.input.Text
	if text == "" {
		return
	}

	// 创建一个Message对象
	msg := protocol.Message{
		Type:    "chat",
		Sender:  ui.username, // 使用在创建UI时传入的用户名
		Payload: text,
	}

	// 通过client核心逻辑将消息发送出去
	ui.client.Send(msg)
	// 清空输入框
	ui.input.SetText("")
}

// Run 启动UI并开始监听来自客户端核心的消息
func (ui *UI) Run() {
	// 启动一个后台goroutine来处理从服务器收到的消息并更新UI
	go func() {
		// 这个循环会一直运行，直到GetIncomingMessages()通道被关闭
		for msg := range ui.client.GetIncomingMessages() {
			// 将消息格式化后添加到数据绑定列表中
			// Fyne的数据绑定是线程安全的，因此可以安全地从任何goroutine调用
			timestampStr := msg.Timestamp.Format("15:04:05") // 格式化为 HH:MM:SS

			formattedMsg := fmt.Sprintf("[%s] %s: %s", timestampStr, msg.Sender, msg.Payload)

			ui.chatHistory.Append(formattedMsg)
		}

		// 当通道关闭后（意味着客户端核心已关闭），可以给出提示或禁用UI
		ui.input.Disable()
		ui.sendButton.Disable()
		ui.chatHistory.Append("--- You have been disconnected ---")
	}()

	// 启动并运行Fyne应用，这将阻塞主goroutine直到窗口关闭
	ui.window.ShowAndRun()
}
