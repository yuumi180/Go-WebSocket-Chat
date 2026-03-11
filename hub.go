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
			// --- 核心改动在这里 ---
			switch msg.Type {
			case "private", "broadcast":
				// --- 处理聊天消息 ---
				// (这部分逻辑与之前相同，可以折叠)
				if msg.Type == "broadcast" {
					chatMsg := ChatMessage{Sender: msg.Sender, Content: msg.Content}
					DB.Create(&chatMsg)
					msg.Timestamp = chatMsg.CreatedAt
					messageBytes, _ = json.Marshal(msg)
				} else { // private message
					msg.Timestamp = time.Now()
					messageBytes, _ = json.Marshal(msg)
				}
				if msg.Type == "private" && msg.To != "" {
					// 私聊分发
					for client := range h.clients {
						if client.name == msg.To || client.name == msg.Sender {
							client.send <- messageBytes
						}
					}
				} else {
					// 广播分发
					for client := range h.clients {
						client.send <- messageBytes
					}
				}
			case "typing", "stop_typing":
				// --- 新增：处理打字状态消息 ---
				// 只转发给指定的目标用户，不广播，不存数据库
				for client := range h.clients {
					if client.name == msg.To {
						// 直接将原始的字节流转发
						select {
						case client.send <- messageBytes:
						default:
							close(client.send)
							delete(h.clients, client)
						}
						break // 找到目标后就不用再遍历了
					}
				}

			default:
				// 处理其他类型的消息，例如 system 消息的广播
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
