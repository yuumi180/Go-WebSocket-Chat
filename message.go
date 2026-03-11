// message.go
package main

import "time"

// ChatMessage 定义了存储在数据库中的聊天消息模型
type ChatMessage struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time

	Sender   string `gorm:"index"` // 发送者，添加索引以优化查询
	Receiver string `gorm:"index"` // 新增：接收者，也添加索引
	Type     string // 新增：消息类型, "broadcast" 或 "private"
	Content  string
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
