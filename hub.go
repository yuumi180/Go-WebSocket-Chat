// hub.go
package main

import (
	"encoding/json"
	"log"
	"time"
)

// Hub 结构体和 newHub 函数保持不变
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

// broadcastUserList 辅助函数，用于获取所有用户和在线用户列表并广播。
func (h *Hub) broadcastUserList() {
	// 1. 获取在线用户列表
	var onlineUserList []string
	for client := range h.clients {
		onlineUserList = append(onlineUserList, client.name)
	}

	// 2. 从数据库获取所有注册用户的用户名列表
	var allUserList []string
	// 使用 Pluck 直接将 'username' 列提取到 allUserList 切片中，效率更高
	DB.Model(&User{}).Pluck("username", &allUserList)

	// 3. 构建并广播消息
	userListMsg := Message{
		Type:      "user_list",
		UserList:  onlineUserList, // 在线的
		AllUsers:  allUserList,    // 所有的
		Timestamp: time.Now(),
	}
	msgBytes, _ := json.Marshal(userListMsg)

	for client := range h.clients {
		select {
		case client.send <- msgBytes:
		default:
			close(client.send)
			delete(h.clients, client)
		}
	}
}

// run 方法保持不变

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true

			// 发送离线消息给刚上线的用户
			go h.sendOfflineMessages(client.name)

			h.broadcastUserList()

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.broadcastUserList()
			}

		case messageBytes := <-h.broadcast:
			var msg Message
			if err := json.Unmarshal(messageBytes, &msg); err != nil {
				continue
			}

			if msg.Type == "broadcast" || msg.Type == "private" {
				// 创建数据库记录
				chatMsg := ChatMessage{
					Sender:   msg.Sender,
					Receiver: msg.To, // 将消息的 To 字段存入 Receiver
					Type:     msg.Type,
					Content:  msg.Content,
				}
				DB.Create(&chatMsg)

				// 使用数据库生成的时间戳更新消息
				msg.Timestamp = chatMsg.CreatedAt
				messageBytes, _ = json.Marshal(msg)
			}

			if msg.Type == "private" && msg.To != "" {
				// 检查目标用户是否在线
				targetOnline := false
				for client := range h.clients {
					if client.name == msg.To {
						targetOnline = true
						select {
						case client.send <- messageBytes:
						default:
							close(client.send)
							delete(h.clients, client)
						}
					}
				}

				// 如果目标用户不在线，消息已存储到数据库，等待其上线后拉取
				if !targetOnline {
					log.Printf("用户 %s 不在线，消息已存储", msg.To)
				}

				// 同时也发送给发送者
				for client := range h.clients {
					if client.name == msg.Sender {
						select {
						case client.send <- messageBytes:
						default:
							close(client.send)
							delete(h.clients, client)
						}
					}
				}
			} else {
				for client := range h.clients {
					select {
					case client.send <- messageBytes:
					default:
						close(client.send)
						delete(h.clients, client)
					}
				}
			}
		}
	}
}

// sendOfflineMessages 发送离线消息给指定用户
func (h *Hub) sendOfflineMessages(username string) {
	var messages []ChatMessage
	// 查询所有发送给该用户的未读私信
	result := DB.Where("receiver = ? AND type = ? AND is_read = ?", username, "private", false).
		Order("created_at asc").
		Find(&messages)

	if result.Error != nil || len(messages) == 0 {
		return
	}

	// 获取当前用户的客户端
	var targetClient *Client
	for client := range h.clients {
		if client.name == username {
			targetClient = client
			break
		}
	}

	if targetClient == nil {
		return
	}

	// 发送离线消息
	for _, msg := range messages {
		offlineMsg := Message{
			Type:      "private",
			Sender:    msg.Sender,
			To:        msg.Receiver,
			Content:   msg.Content,
			Timestamp: msg.CreatedAt,
		}
		msgBytes, _ := json.Marshal(offlineMsg)

		select {
		case targetClient.send <- msgBytes:
		default:
			return
		}

		// 标记为已读
		DB.Model(&msg).Update("is_read", true)
	}

	log.Printf("已向用户 %s 推送 %d 条离线消息", username, len(messages))
}

// ... existing code ...
