// message.go
package main

import "time"

// ChatMessage 定义了存储在数据库中的聊天消息模型
type ChatMessage struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time

	Sender   string `gorm:"index:idx_sender"` // 发送者
	Receiver string `gorm:"index:idx_receiver"` // 接收者
	Type     string `gorm:"index:idx_type"`   // 消息类型
	Content  string
	IsRead   bool `gorm:"default:false;index:idx_is_read"` // 是否已读
}

// Message 定义了在 WebSocket 中传输的消息结构
type Message struct {
	Type        string    `json:"type"` // "broadcast", "private", "system", "user_list", "typing", "stop_typing"
	Sender      string    `json:"sender,omitempty"`
	To          string    `json:"to,omitempty"`
	Content     string    `json:"content,omitempty"`
	ContentType string    `json:"contentType,omitempty"` // 新增: "text" 或 "image"
	UserList    []string  `json:"user_list,omitempty"`   // 在线用户列表
	AllUsers    []string  `json:"all_users,omitempty"`   // 新增：所有注册用户列表
	Timestamp   time.Time `json:"timestamp,omitempty"`
}
