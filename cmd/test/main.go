package main

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// UI 结构体持有对窗口的引用
type UI struct {
	window fyne.Window
}

// createContentView 创建包含两个按钮的视图
// 一个会触发错误，另一个会正常工作
func (ui *UI) createContentView() fyne.CanvasObject {

	infoLabel := widget.NewLabel("点击下面的按钮来测试")
	infoLabel.Alignment = fyne.TextAlignCenter

	// --- 按钮 1: 错误的方式 ---
	// 这个按钮会直接在新的 Goroutine 中调用 CenterOnScreen()
	errorButton := widget.NewButton("在 Goroutine 中居中 (会出错)", func() {
		infoLabel.SetText("将在2秒后尝试错误地居中...")

		// 启动一个后台 Goroutine 模拟耗时操作
		go func() {
			fmt.Println("后台任务开始 (错误的方式)...")
			time.Sleep(2 * time.Second) // 模拟网络请求或计算

			// !!! 错误的操作 !!!
			// 直接在非 GUI 线程中修改 UI (窗口)
			// 这会导致竞争条件，很可能使程序崩溃 (panic)
			fmt.Println("正在从后台 Goroutine 直接调用 CenterOnScreen()...")
			ui.window.CenterOnScreen()

			fmt.Println("后台任务结束 (如果程序还没崩溃的话)")
		}()
	})

	// --- 按钮 2: 正确的方式 ---
	// 这个按钮会使用 fyne.Do() 来安全地更新 UI
	correctButton := widget.NewButton("在 Goroutine 中安全居中 (正确)", func() {
		infoLabel.SetText("将在2秒后安全地居中...")

		// 启动一个后台 Goroutine
		go func() {
			fmt.Println("后台任务开始 (正确的方式)...")
			time.Sleep(2 * time.Second) // 模拟耗时操作

			// *** 正确的操作 ***
			// 将 UI 更新代码包裹在 fyne.Do() 中
			// Fyne 会确保这个函数在主 GUI 线程上被执行
			fmt.Println("正在通过 fyne.Do() 安排 CenterOnScreen() 的调用...")
			fyne.Do(func() {
				fmt.Println("fyne.Do() 开始执行，现在在主 GUI 线程上。")
				ui.window.CenterOnScreen()
				infoLabel.SetText("窗口已安全居中！")
			})
			fmt.Println("后台任务结束 (正确的方式)。")
		}()
	})

	// 将组件放入容器中
	return container.NewVBox(
		infoLabel,
		errorButton,
		correctButton,
	)
}

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("Goroutine UI 操作示例")
	myWindow.Resize(fyne.NewSize(450, 200))
	// 先居中一次，方便观察效果
	// myWindow.CenterOnScreen()

	ui := &UI{window: myWindow}

	// 创建并设置内容
	view := ui.createContentView()
	myWindow.SetContent(view)

	myWindow.ShowAndRun()
}
