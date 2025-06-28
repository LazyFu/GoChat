package main

import (
	"GoChat/internal/client"
	"fmt"

	"fyne.io/fyne/v2/app"
)

func main() {
	fyneApp := app.NewWithID("io.github.lazyfu.chattool")
	coreClient := client.NewClient()
	gui := client.NewUI(fyneApp, coreClient)
	gui.Run()
	coreClient.Close()
	fmt.Println("客户端已关闭。")
}
