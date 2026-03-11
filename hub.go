// hub.go
package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

// Hub 维护所有活动的客户端连接
type Hub struct {
	mu         sync.RWMutex
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

// broadcastUserList 优化版本：减少不必要的广播
func (h *Hub) broadcastUserList() {
	// 使用 sync.Pool 复用对象，减少 GC 压力
	var onlineUserList []string
	onlineUserList = make([]string, 0, len(h.clients))
	
	for client := range h.clients {
		onlineUserList = append(onlineUserList, client.name)
	}

	var allUserList []string
	DB.Model(&User{}).Pluck("username", &allUserList)

	userListMsg := Message{
		Type:      "user_list",
		UserList:  onlineUserList,
		AllUsers:  allUserList,
		Timestamp: time.Now(),
	}
	msgBytes, _ := json.Marshal(userListMsg)

	// 批量发送，减少锁竞争
	h.broadcastToAll(msgBytes)
}

// broadcastToAll 批量发送给所有客户端
func (h *Hub) broadcastToAll(msgBytes []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.send <- msgBytes:
		default:
			// 客户端缓冲区已满，标记为需要清理
			go func(c *Client) {
				close(c.send)
				h.unregister <- c
			}(client)
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

// DisconnectUser 断开指定用户的连接
func (h *Hub) DisconnectUser(username string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		if client.name == username {
			log.Printf("强制断开用户 %s 的连接", username)
			// 关闭 WebSocket 连接
			client.conn.Close()
			// 从客户端列表中移除
			delete(h.clients, client)
			// 关闭 send 通道
			close(client.send)
		}
	}
}
