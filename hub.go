// hub.go
package main

import (
	"encoding/json"
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

			if msg.Type == "private" || msg.Type == "broadcast" {
				if msg.Type == "broadcast" {
					chatMsg := ChatMessage{Sender: msg.Sender, Content: msg.Content}
					DB.Create(&chatMsg)
					msg.Timestamp = chatMsg.CreatedAt
				} else {
					msg.Timestamp = time.Now()
				}
				messageBytes, _ = json.Marshal(msg)
			}

			if msg.Type == "private" && msg.To != "" {
				for client := range h.clients {
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
