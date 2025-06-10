package protocol

import (
	"encoding/json"
	"time"
)

// 定义消息类型常量
const (
	LoginRequest   = "LoginRequest"
	TextMessage    = "TextMessage"
	UserListUpdate = "UserListUpdate"
	PrivateMessage = "PrivateMessage"
	TreeUpdate     = "tree_update"
	// 群聊
	GroupMessage       = "GroupMessage"
	CreateGroupRequest = "CreateGroupRequest"
	JoinGroupRequest   = "JoinGroupRequest"
	LeaveGroupRequest  = "LeaveGroupRequest"
	GroupNotification  = "GroupNotification"
)

type TreeData struct {
	Users  []string            `json:"users"`
	Groups map[string][]string `json:"groups"` // key是群名, value是成员列表
}

// Message represents a basic chat message structure.
type Message struct {
	Type        string    `json:"type"`                   // 消息类型
	Payload     string    `json:"payload"`                // 消息内容
	TreePayload TreeData  `json:"tree_payload,omitempty"` // 树形结构的消息内容
	Sender      string    `json:"sender"`                 // 发送者
	Recipient   string    `json:"recipient,omitempty"`    // 接收者
	GroupName   string    `json:"groupname,omitempty"`    // 群组名称
	UserList    []string  `json:"userlist"`               // 用户列表
	Timestamp   time.Time `json:"timestamp"`              // 时间戳
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
