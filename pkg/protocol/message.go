package protocol

import (
	"encoding/json"
	"time"
)

// 定义消息类型常量
const (
	LoginRequest       = "cmd_login"
	CreateGroupRequest = "cmd_create_group"
	JoinGroupRequest   = "cmd_join_group"
	LeaveGroupRequest  = "cmd_leave_group"

	// --- 数据/通知类型 ---
	TreeUpdate         = "data_tree_update" // 树状列表更新
	BroadcastMessage   = "msg_broadcast"    // 广播消息
	PrivateMessage     = "msg_private"      // 私聊消息
	GroupMessage       = "msg_group"        // 群聊消息
	PrivateFileMessage = "file_private"     // 私聊文件
	GroupFileMessage   = "file_group"       // 群聊文件
)

type TreePayload struct {
	Users  []string            `json:"users"`  // 在线用户列表
	Groups map[string][]string `json:"groups"` // 群组列表，键为群组名，值为成员列表
}

type FilePayload struct {
	Name string `json:"name"` // 文件名
	Size int64  `json:"size"` // 文件大小
	Data []byte `json:"data"` // 文件内容
}

type Message struct {
	Type      string    `json:"type"`      // 消息类型
	Sender    string    `json:"sender"`    // 发送者
	Timestamp time.Time `json:"timestamp"` // 时间戳

	Recipient   string      `json:"recipient,omitempty"`    // 接收者
	GroupName   string      `json:"groupname,omitempty"`    // 群组名称
	TextPayload string      `json:"text_payload,omitempty"` // 文本内容
	FilePayload FilePayload `json:"file_payload"`           // 文件内容
	TreePayload TreePayload `json:"tree_payload,omitempty"` // 树状结构数据
}

// Serialize 将 Message 序列化为 JSON 字符串
func (m *Message) Serialize() (string, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Deserialize 将 JSON 字符串反序列化为 Message
func Deserialize(data string) (*Message, error) {
	var msg Message
	err := json.Unmarshal([]byte(data), &msg)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}
