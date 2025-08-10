# Go Chat

使用go开发的局域网聊天软件，支持一对一私聊，群聊，传文件。

## 开始

运行测试`go run ./cmd/client`，`go run ./cmd/server`

在release中下载exe, 适用于x64 windows, 先运行server, 再运行client, 在client界面中输入运行server的设备IP, 8080端口, 例如`127.0.0.1:8080`.

## NOTE

>Do not use the `centerOnScreen` in fyne.Do.

## Problems

- 发送消息超出一行长度会显示不全
- 输入框中粘贴问题
- 传输没用加密功能，因为ai告诉我这会很麻烦
- 群组没有用户列表
- build之后的exe程序运行时会显示终端
