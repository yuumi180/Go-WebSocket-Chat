// hub.go
package main

import (
	"encoding/json"
	"time"
)

// Hub 维护所有活跃的客户端，并向所有客户端广播消息。
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

// newHub 创建并返回一个新的 Hub 实例。
func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}
func (h *Hub) broadcastUserList() {
	var userList []string
	for client := range h.clients {
		userList = append(userList, client.name)
	}
	userListMsg := Message{
		Type:      "user_list",
		UserList:  userList,
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

// --- 这是核心修正 ---
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
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
			// 统一为所有聊天消息添加时间戳并存入数据库
			if msg.Type == "broadcast" || msg.Type == "private" {
				// 私聊消息不应该存入公共历史记录，这里我们先简化，
				// 假设所有消息都存，或者可以加个 if 判断
				if msg.Type == "broadcast" {
					chatMsg := ChatMessage{
						Sender:  msg.Sender,
						Content: msg.Content,
					}
					DB.Create(&chatMsg)
					msg.Timestamp = chatMsg.CreatedAt
				} else {
					// 私聊消息使用当前服务器时间
					msg.Timestamp = time.Now()
				}

				// 重新序列化带有时间戳的消息
				messageBytes, _ = json.Marshal(msg)
			}

			// --- 统一的消息分发逻辑 ---
			if msg.Type == "private" && msg.To != "" {
				// 私聊分发
				for client := range h.clients {
					// 发送给目标和发送者自己
					if client.name == msg.To || client.name == msg.Sender {
						select {
						case client.send <- messageBytes:
						default:
							close(client.send)
							delete(h.clients, client)
						}
					}
				}
			} else {
				// 广播分发 (包括 system 和 broadcast)
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
