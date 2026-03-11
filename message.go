// message.go
package main

import "time"

// ChatMessage 定义了存储在数据库中的聊天消息模型
type ChatMessage struct {
	// gorm.Model 包含了 ID, CreatedAt, UpdatedAt, DeletedAt
	// 我们在这里不需要它，因为我们只需要基本字段
	// 如果需要，可以取消注释下面的 gorm.Model
	// gorm.Model
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	Sender    string
	Content   string
}

// Message 定义了在 WebSocket 中传输的消息结构
type Message struct {
	Type      string    `json:"type"`                // 消息类型: "broadcast", "system", "user_list"
	Sender    string    `json:"sender,omitempty"`    // 发送者
	To        string    `json:"to,omitempty"`        // 新增：消息接收者 (用于私聊)
	Content   string    `json:"content,omitempty"`   // 消息内容
	UserList  []string  `json:"user_list,omitempty"` // 在线用户列表
	Timestamp time.Time `json:"timestamp,omitempty"` // 消息时间戳
}
