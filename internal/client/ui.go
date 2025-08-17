package client

import (
	"GoChat/pkg/protocol"
	"fmt"
	"slices"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type customEntry struct {
	widget.Entry
	app fyne.App
}

func newCustomEntry(app fyne.App) *customEntry {
	entry := &customEntry{
		app: app,
	}
	entry.ExtendBaseWidget(entry)
	return entry
}

func (e *customEntry) TappedSecondary(pe *fyne.PointEvent) {

	clipboard := e.app.Clipboard()

	pasteItem := fyne.NewMenuItem("粘贴", func() {
		e.SetText(e.Text + clipboard.Content())
	})

	menu := fyne.NewMenu("", pasteItem)
	widget.ShowPopUpMenuAtPosition(menu, e.app.Driver().CanvasForObject(e), pe.AbsolutePosition)
}

type UI struct {
	client *Client
	app    fyne.App
	window fyne.Window

	accordion *widget.Accordion
	chatTabs  *container.DocTabs

	username          string
	usersListBinding  binding.StringList
	groupsListBinding binding.StringList

	chatHistories      map[string]binding.StringList
	chatHistoriesMutex sync.Mutex
}

// NewUI 创建并初始化UI
func NewUI(app fyne.App, c *Client) *UI {
	w := app.NewWindow("Go Chat")
	w.SetMaster()

	ui := &UI{
		client:            c,
		app:               app,
		window:            w,
		chatHistories:     make(map[string]binding.StringList),
		usersListBinding:  binding.NewStringList(),
		groupsListBinding: binding.NewStringList(),
	}

	w.SetOnClosed(func() { app.Quit() })
	return ui
}

// Run 启动并显示UI
func (ui *UI) Run() {
	loginView := ui.createLoginView()
	ui.window.SetContent(loginView)
	ui.window.Resize(fyne.NewSize(400, 200))
	ui.window.CenterOnScreen()
	ui.window.ShowAndRun()
}

// createLoginView 创建登录视图
func (ui *UI) createLoginView() fyne.CanvasObject {
	serverAddrEntry := widget.NewEntry()
	serverAddrEntry.SetPlaceHolder("输入服务器地址，如: 127.0.0.1:8080")
	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("输入用户名")
	statusLabel := widget.NewLabel("")

	var loginButton *widget.Button
	loginButton = widget.NewButton("登录", func() {
		username := usernameEntry.Text
		server := serverAddrEntry.Text

		// 校验用户名
		if username == "" {
			dialog.ShowError(fmt.Errorf("用户名不能为空"), ui.window)
			return
		}

		// 校验服务器地址
		if server == "" {
			dialog.ShowError(fmt.Errorf("服务器地址不能为空"), ui.window)
			return
		}

		loginButton.Disable()
		statusLabel.SetText("正在连接服务器...")

		go func() {
			if err := ui.client.Connect(server); err != nil {
				fyne.Do(func() {
					dialog.ShowError(err, ui.window)
					loginButton.Enable()
					statusLabel.SetText("连接失败")
				})
				return
			}

			fyne.Do(func() {
				ui.switchToChatView(username)
			})
		}()
	})

	return container.NewCenter(container.NewVBox(
		widget.NewLabel("欢迎来到聊天室"),
		serverAddrEntry,
		usernameEntry,
		loginButton,
		statusLabel,
	))
}

// switchToChatView 负责创建主聊天界面
func (ui *UI) switchToChatView(username string) {
	ui.username = username
	ui.client.SetUsername(username)
	ui.window.SetTitle(fmt.Sprintf("Go Chat - %s", ui.username))

	ui.accordion = ui.createAccordion()
	createGroupBtn := widget.NewButton("创建群组", ui.showCreateGroupDialog)
	leftPanel := container.NewBorder(nil, createGroupBtn, nil, nil, ui.accordion)

	ui.chatTabs = container.NewDocTabs()
	ui.chatTabs.OnClosed = func(item *container.TabItem) {
		name := item.Text
		fmt.Printf("标签页 '%s' 已关闭\n", name)

		ui.chatHistoriesMutex.Lock()
		delete(ui.chatHistories, name)
		ui.chatHistoriesMutex.Unlock()

		if ui.isGroup(name) {
			leaveMsg := protocol.Message{
				Type:      protocol.LeaveGroupRequest,
				Sender:    ui.username,
				GroupName: name,
			}
			ui.client.Send(leaveMsg)
		}
	}
	ui.openChatTab("世界大厅")

	split := container.NewHSplit(leftPanel, ui.chatTabs)
	split.SetOffset(0.25)

	ui.window.SetContent(split)
	ui.window.Resize(fyne.NewSize(900, 600))

	ui.client.Start()
	ui.startBackgroundTasks()

	loginMsg := protocol.Message{Type: protocol.LoginRequest, Sender: ui.username}
	ui.client.Send(loginMsg)
}

// openChatTab 确保一个聊天标签页被创建并选中
func (ui *UI) openChatTab(name string) {
	for _, tab := range ui.chatTabs.Items {
		if tab.Text == name {
			ui.chatTabs.Select(tab)
			return
		}
	}

	content := ui.createChatTabContent(name)
	newTab := container.NewTabItem(name, content)
	ui.chatTabs.Append(newTab)

	// TODO:对“世界大厅”应用特殊规则。
	// if name == "世界大厅" {
	// 	// 使用 DocTabs 的 SetTabClosable 方法来使其不可关闭。
	// 	ui.chatTabs.SetTabClosable(newTab, false)
	// }

	ui.chatTabs.Select(newTab)
}

// createChatTabContent 创建一个聊天标签页的内容
func (ui *UI) createChatTabContent(name string) fyne.CanvasObject {
	historyBinding := binding.NewStringList()
	ui.chatHistoriesMutex.Lock()
	ui.chatHistories[name] = historyBinding
	ui.chatHistoriesMutex.Unlock()

	historyList := widget.NewListWithData(historyBinding,
		func() fyne.CanvasObject {
			label := widget.NewLabel("template")
			label.Wrapping = fyne.TextWrapWord
			return label
		},
		func(i binding.DataItem, o fyne.CanvasObject) {
			o.(*widget.Label).Bind(i.(binding.String))
		},
	)

	input := newCustomEntry(ui.app)
	input.SetPlaceHolder("在这里输入消息...")

	sendBtn := widget.NewButton("发送", func() {
		if input.Text == "" {
			return
		}
		var msgType, recipient, groupName string
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
	fileBtn := widget.NewButtonWithIcon("", theme.FileIcon(), func() {
		ui.showFileOpenDialog(name)
	})
	inputBox := container.NewBorder(nil, nil, nil, container.NewHBox(sendBtn, fileBtn), input)
	input.OnSubmitted = func(_ string) { sendBtn.OnTapped() }

	return container.NewBorder(nil, inputBox, nil, nil, historyList)
}

func (ui *UI) createAccordion() *widget.Accordion {
	usersList := widget.NewListWithData(ui.usersListBinding,
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i binding.DataItem, o fyne.CanvasObject) { o.(*widget.Label).Bind(i.(binding.String)) },
	)
	usersList.OnSelected = func(id widget.ListItemID) {
		selectedUsername, _ := ui.usersListBinding.GetValue(id)
		usersList.Unselect(id)
		fmt.Printf("准备与 %s 私聊...\n", selectedUsername)
		ui.openChatTab(selectedUsername)
	}

	groupsList := widget.NewListWithData(ui.groupsListBinding,
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i binding.DataItem, o fyne.CanvasObject) { o.(*widget.Label).Bind(i.(binding.String)) },
	)
	groupsList.OnSelected = func(id widget.ListItemID) {
		groupName, _ := ui.groupsListBinding.GetValue(id)
		groupsList.Unselect(id)
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

	userAccordionItem := widget.NewAccordionItem("在线用户", usersList)
	groupAccordionItem := widget.NewAccordionItem("可用群组", groupsList)
	accordion := widget.NewAccordion(userAccordionItem, groupAccordionItem)
	accordion.Open(0)
	return accordion
}

func (ui *UI) startBackgroundTasks() {
	go func() {
		for msg := range ui.client.GetIncomingMessages() {
			localMsg := msg
			fyne.Do(func() {
				switch localMsg.Type {
				case protocol.TreeUpdate:
					var otherUsers []string
					for _, user := range localMsg.TreePayload.Users {
						if user != ui.username {
							otherUsers = append(otherUsers, user)
						}
					}
					ui.usersListBinding.Set(otherUsers)

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
					var conversationPartner string
					if localMsg.Sender == ui.username {
						conversationPartner = localMsg.Recipient
					} else {
						conversationPartner = localMsg.Sender
					}
					ui.addMessage(conversationPartner, localMsg)
				case protocol.PrivateFileMessage, protocol.GroupFileMessage:
					if localMsg.Sender != ui.username {
						fileInfo := localMsg.FilePayload
						dialog.ShowConfirm("接收文件",
							fmt.Sprintf("来自 %s 的文件: %s (大小: %.2f KB)\n您想保存吗？",
								localMsg.Sender, fileInfo.Name, float64(fileInfo.Size)/1024),
							func(save bool) {
								if !save {
									return
								}
								ui.showFileSaveDialog(fileInfo, localMsg.GroupName, localMsg.Sender)
							}, ui.window)
					}
				}
			})
		}
		fyne.Do(func() {
			dialog.ShowInformation("连接断开", "您已与服务器断开连接。", ui.window)
			ui.window.SetContent(ui.createLoginView())
			ui.window.Resize(fyne.NewSize(400, 200))
		})
	}()
}

func (ui *UI) showCreateGroupDialog() {
	entry := widget.NewEntry()
	dialog.ShowForm("创建新群组", "创建", "取消", []*widget.FormItem{
		widget.NewFormItem("群组名", entry),
	}, func(create bool) {
		if !create || entry.Text == "" {
			return
		}
		ui.client.SendChatMessage(protocol.CreateGroupRequest, "", "", entry.Text)
	}, ui.window)
}

func (ui *UI) addMessage(tabName string, msg protocol.Message) {
	ui.chatHistoriesMutex.Lock()
	history, ok := ui.chatHistories[tabName]
	ui.chatHistoriesMutex.Unlock()

	if !ok {
		ui.openChatTab(tabName)
		ui.chatHistoriesMutex.Lock()
		history = ui.chatHistories[tabName]
		ui.chatHistoriesMutex.Unlock()
	}

	if history != nil {
		timestampStr := msg.Timestamp.Format("15:04:05")
		formattedMsg := fmt.Sprintf("[%s] %s: %s", timestampStr, msg.Sender, msg.TextPayload)
		history.Append(formattedMsg)
	}
}

func (ui *UI) isGroup(name string) bool {
	list, _ := ui.groupsListBinding.Get()
	return slices.Contains(list, name)
}

func (ui *UI) showFileOpenDialog(targetName string) {
	dialog.ShowFileOpen(func(readCloser fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, ui.window)
			return
		}
		if readCloser == nil {
			return
		}

		filePath := readCloser.URI().Path()

		var msgType string
		if ui.isGroup(targetName) {
			msgType = protocol.GroupFileMessage
		} else {
			msgType = protocol.PrivateFileMessage
		}

		ui.client.SendFile(msgType, targetName, targetName, filePath)

		systemMsg := protocol.Message{
			Timestamp:   time.Now(),
			Sender:      "系统",
			TextPayload: fmt.Sprintf("正在向 %s 发送文件: %s...", targetName, readCloser.URI().Name()),
		}
		ui.addMessage(targetName, systemMsg)

	}, ui.window)
}

func (ui *UI) showFileSaveDialog(fileInfo protocol.FilePayload, tabName string, senderName string) {
	saveDialog := dialog.NewFileSave(func(writeCloser fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, ui.window)
			return
		}
		if writeCloser == nil {
			return
		}

		savePath := writeCloser.URI().Path()
		_ = writeCloser.Close()
		ui.client.SaveFile(fileInfo, savePath)

		systemMsg := protocol.Message{
			Timestamp:   time.Now(),
			Sender:      "系统",
			TextPayload: fmt.Sprintf("正在保存来自 %s 的文件: %s...", senderName, fileInfo.Name),
		}
		ui.addMessage(tabName, systemMsg)

	}, ui.window)

	saveDialog.SetFileName(fileInfo.Name)
	saveDialog.Show()
}
