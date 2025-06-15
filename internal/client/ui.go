// File: internal/client/ui.go
package client

import (
	"ChatTool/pkg/protocol" // 请确认您的模块名
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// UI 封装了所有UI组件和状态
type UI struct {
	client *Client
	app    fyne.App
	window fyne.Window

	// UI 组件
	accordion *widget.Accordion // 用手风琴代替树
	chatTabs  *container.AppTabs

	// 状态数据和数据绑定
	username          string
	usersListBinding  binding.StringList // 专门为用户列表提供数据
	groupsListBinding binding.StringList // 专门为群组列表提供数据

	// 每个聊天标签页的数据源
	chatHistories      map[string]binding.StringList
	chatHistoriesMutex sync.Mutex
}

// NewUI 创建并初始化UI
func NewUI(app fyne.App, c *Client) *UI {
	w := app.NewWindow("Go Chat")
	w.SetMaster() // 设置为主窗口

	ui := &UI{
		client:            c,
		app:               app,
		window:            w,
		chatHistories:     make(map[string]binding.StringList),
		usersListBinding:  binding.NewStringList(),
		groupsListBinding: binding.NewStringList(),
	}

	w.SetOnClosed(func() {
		// 当主窗口关闭时，退出整个应用
		app.Quit()
	})

	return ui
}

// Run 启动并显示UI
func (ui *UI) Run() {
	// 初始界面就是登录界面
	loginView := ui.createLoginView()
	ui.window.SetContent(loginView)
	ui.window.Resize(fyne.NewSize(400, 200)) // 登录窗口小一点

	// ShowAndRun会阻塞，直到app.Quit()被调用
	ui.window.ShowAndRun()
}

// createLoginView 创建登录视图
func (ui *UI) createLoginView() fyne.CanvasObject {
	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("输入用户名")
	statusLabel := widget.NewLabel("")

	var loginButton *widget.Button // 先声明变量

	loginButton = widget.NewButton("登录", func() {
		username := usernameEntry.Text
		if username == "" {
			dialog.ShowError(fmt.Errorf("用户名不能为空"), ui.window)
			return
		}

		// 禁用登录按钮，防止重复点击
		loginButton.Disable()
		statusLabel.SetText("正在连接服务器...")

		// 在后台处理连接
		go func() {
			// 连接服务器
			if err := ui.client.Connect("127.0.0.1:8080"); err != nil {
				// 连接失败，回到主线程更新UI
				fyne.Do(func() {
					dialog.ShowError(err, ui.window)
					loginButton.Enable()
					statusLabel.SetText("连接失败")
				})
				return
			}

			fmt.Println("服务器连接成功")

			// 设置用户名
			ui.client.SetUsername(username)
			ui.username = username

			// 重要：先启动客户端循环，确保有goroutine处理outgoing通道
			ui.client.Start()

			// 构造登录消息
			loginMsg := protocol.Message{
				Type:   protocol.LoginRequest,
				Sender: username,
			}

			fmt.Println("发送登录消息:", username)
			// 发送登录消息 - 这个方法不返回错误
			ui.client.Send(loginMsg)

			// 启动后台任务监听服务器消息
			ui.startBackgroundTasks()

			// 在UI线程切换界面
			fyne.Do(func() {
				ui.prepareAndShowChatView()
			})
		}()
	})

	return container.NewCenter(container.NewVBox(
		widget.NewLabel("欢迎来到聊天室"),
		usernameEntry,
		loginButton,
		statusLabel,
	))
}

// 新增：将UI切换逻辑与网络操作分离
func (ui *UI) prepareAndShowChatView() {
	// 准备UI元素
	ui.accordion = ui.createAccordion()
	createGroupBtn := widget.NewButton("创建群组", ui.showCreateGroupDialog)
	leftPanel := container.NewBorder(nil, createGroupBtn, nil, nil, ui.accordion)

	ui.chatTabs = container.NewAppTabs()
	ui.openChatTab("世界大厅")

	split := container.NewHSplit(leftPanel, ui.chatTabs)
	split.SetOffset(0.25)

	// 先改变窗口大小
	ui.window.Resize(fyne.NewSize(900, 600))

	// 再设置内容
	ui.window.SetContent(split)

	// 最后居中和设置标题
	ui.window.SetTitle(fmt.Sprintf("Go Chat - %s", ui.username))
}

// createAccordion 创建并返回一个手风琴布局
func (ui *UI) createAccordion() *widget.Accordion {
	// --- 创建用户列表 ---
	usersList := widget.NewListWithData(ui.usersListBinding,
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i binding.DataItem, o fyne.CanvasObject) { o.(*widget.Label).Bind(i.(binding.String)) },
	)
	usersList.OnSelected = func(id widget.ListItemID) {
		// 当用户被选中时，可以发起私聊
		username, _ := ui.usersListBinding.GetValue(id)
		fmt.Printf("准备与 %s 私聊...\n", username)
		usersList.Unselect(id) // 取消选中状态
		ui.openChatTab(username)
	}

	// --- 创建群组列表 ---
	groupsList := widget.NewListWithData(ui.groupsListBinding,
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i binding.DataItem, o fyne.CanvasObject) { o.(*widget.Label).Bind(i.(binding.String)) },
	)
	groupsList.OnSelected = func(id widget.ListItemID) {
		groupName, _ := ui.groupsListBinding.GetValue(id)
		dialog.ShowConfirm("加入群组", fmt.Sprintf("您想加入群组 '%s' 吗？", groupName), func(join bool) {
			if !join {
				return
			}
			joinMsg := protocol.Message{
				Type:      protocol.JoinGroupRequest,
				Sender:    ui.username,
				GroupName: groupName,
			}
			ui.client.Send(joinMsg)
			ui.openChatTab(groupName)
		}, ui.window)
	}

	// --- 创建手风琴项目并组合 ---
	userAccordionItem := widget.NewAccordionItem("在线用户", usersList)
	groupAccordionItem := widget.NewAccordionItem("可用群组", groupsList)

	// 创建手风琴，并默认打开“在线用户”
	accordion := widget.NewAccordion(userAccordionItem, groupAccordionItem)
	accordion.Open(0) // 索引为0的项，即用户列表

	return accordion
}

// startBackgroundTasks 启动后台任务，处理来自服务器的消息
func (ui *UI) startBackgroundTasks() {
	fmt.Println("启动后台任务，开始监听服务器消息")
	go func() {
		for msg := range ui.client.GetIncomingMessages() {
			// 为避免UI更新过于频繁导致卡顿，可以考虑批量更新
			localMsg := msg // 创建局部变量避免闭包问题

			fyne.Do(func() {
				switch localMsg.Type {
				case protocol.TreeUpdate:
					// 更新用户列表
					var otherUsers []string
					for _, user := range localMsg.TreePayload.Users {
						if user != ui.username {
							otherUsers = append(otherUsers, user)
						}
					}
					ui.usersListBinding.Set(otherUsers)

					// 更新群组列表
					var groupNames []string
					for name := range localMsg.TreePayload.Groups {
						groupNames = append(groupNames, name)
					}
					ui.groupsListBinding.Set(groupNames)

				case protocol.BroadcastMessage:
					ui.addMessage("世界大厅", localMsg)

				case protocol.GroupMessage:
					ui.addMessage(localMsg.GroupName, localMsg)

				case protocol.PrivateMessage:
					// 判断这条私聊消息的“对话方”是谁
					var conversationPartner string
					if msg.Sender == ui.username {
						// 如果我是发送者，那么对话方是接收者
						conversationPartner = msg.Recipient
					} else {
						// 如果我是接收者，那么对话方是发送者
						conversationPartner = msg.Sender
					}
					// 将消息添加到以“对话方”命名的标签页中
					ui.openChatTab(conversationPartner)
					ui.addMessage(conversationPartner, msg)
				}
			})
		}

		// 连接断开处理
		fyne.Do(func() {
			dialog.ShowInformation("连接断开", "您已与服务器断开连接。", ui.window)
			ui.window.SetContent(ui.createLoginView())
			ui.window.Resize(fyne.NewSize(400, 200))
		})
	}()
}

// --- UI 辅助函数 ---

// showCreateGroupDialog 显示创建群组的对话框
func (ui *UI) showCreateGroupDialog() {
	entry := widget.NewEntry()
	dialog.ShowForm("创建新群组", "创建", "取消", []*widget.FormItem{
		widget.NewFormItem("群组名", entry),
	}, func(create bool) {
		if !create || entry.Text == "" {
			return
		}
		ui.client.SendChatMessage(protocol.CreateGroupRequest, "", entry.Text, "Create group: "+entry.Text)
	}, ui.window)
}

func (ui *UI) openChatTab(name string) {
	// 检查标签页是否已存在
	for _, tab := range ui.chatTabs.Items {
		if tab.Text == name {
			ui.chatTabs.Select(tab)
			return
		}
	}

	// 如果不存在，创建新的聊天组件和数据源
	historyBinding := binding.NewStringList()

	// 将新的数据源存入map，以便 addMessage 函数可以找到它
	ui.chatHistoriesMutex.Lock()
	ui.chatHistories[name] = historyBinding
	ui.chatHistoriesMutex.Unlock()

	// --- 创建UI组件 ---
	historyList := widget.NewListWithData(historyBinding,
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i binding.DataItem, o fyne.CanvasObject) { o.(*widget.Label).Bind(i.(binding.String)) },
	)
	input := widget.NewEntry()
	input.SetPlaceHolder("在这里输入消息...")

	sendBtn := widget.NewButton("发送", func() {
		if input.Text == "" {
			return
		}

		msgType := protocol.BroadcastMessage
		recipient := ""
		groupName := ""

		if name == "世界大厅" {
			msgType = protocol.BroadcastMessage
		} else if ui.isGroup(name) {
			msgType = protocol.GroupMessage
			groupName = name
		} else {
			msgType = protocol.PrivateMessage
			recipient = name
		}

		ui.client.SendChatMessage(msgType, recipient, groupName, input.Text)
		input.SetText("")
	})
	input.OnSubmitted = func(_ string) { sendBtn.OnTapped() }

	chatContainer := container.NewBorder(nil, container.NewBorder(nil, nil, nil, sendBtn, input), nil, nil, historyList)

	// 创建并添加新标签页
	newTab := container.NewTabItem(name, chatContainer)
	ui.chatTabs.Append(newTab)
	ui.chatTabs.Select(newTab)
}

// addMessage 将消息添加到指定的标签页。
// 如果标签页不存在，它会自动创建、添加并选中该标签页。
func (ui *UI) addMessage(tabName string, msg protocol.Message) {
	ui.chatHistoriesMutex.Lock()
	history, ok := ui.chatHistories[tabName]
	ui.chatHistoriesMutex.Unlock()

	// 如果聊天记录的数据源不存在，说明UI上还没有这个标签页
	if !ok {
		// 自动创建并切换到这个UI标签页
		ui.openChatTab(tabName)
		// 再次获取，这次一定能拿到
		ui.chatHistoriesMutex.Lock()
		history = ui.chatHistories[tabName]
		ui.chatHistoriesMutex.Unlock()
	}

	// 追加消息
	timestampStr := msg.Timestamp.Format("15:04:05")
	formattedMsg := fmt.Sprintf("[%s] %s: %s", timestampStr, msg.Sender, msg.TextPayload)
	history.Append(formattedMsg)
}

func (ui *UI) isGroup(name string) bool {
	// 检查群组列表中是否存在该群组名
	ui.chatHistoriesMutex.Lock()
	defer ui.chatHistoriesMutex.Unlock()
	_, exists := ui.chatHistories[name]
	return exists
}
