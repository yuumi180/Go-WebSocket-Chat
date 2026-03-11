// hub.go
package main

import (
	"encoding/json"
	"time"
)

// Hub 维护所有活跃的客户端，并向所有客户端广播消息。
type Hub struct {
	// 注册的客户端。我们使用一个 map，键是客户端的指针，值是 bool 类型（true 表示存在）。
	clients map[*Client]bool

	// 从客户端传入的消息，需要被广播。
	broadcast chan []byte

	// 来自客户端的注册请求。
	register chan *Client

	// 来自客户端的注销请求。
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

// broadcastUserList 辅助函数，用于获取当前用户列表并广播。
func (h *Hub) broadcastUserList() {
	var userList []string
	for client := range h.clients {
		userList = append(userList, client.name)
	}

	userListMsg := Message{
		Type:      "user_list",
		UserList:  userList,
		Timestamp: time.Now(), // 也给用户列表消息加上时间戳
	}
	msgBytes, _ := json.Marshal(userListMsg)

	// 直接广播，不通过 broadcast channel
	for client := range h.clients {
		select {
		case client.send <- msgBytes:
		default:
			// 如果发送失败，则移除该客户端
			close(client.send)
			delete(h.clients, client)
		}
	}
}

// run 方法启动 Hub 的所有工作。它是一个无限循环，监听并处理来自各个 channel 的请求。
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			// 当有新用户注册时，广播最新的用户列表
			h.broadcastUserList()

		case client := <-h.unregister:
			// 检查客户端是否存在
			if _, ok := h.clients[client]; ok {
				// 先从 clients map 中删除
				delete(h.clients, client)
				// 然后关闭其发送通道
				close(client.send)
				// 最后广播更新后的用户列表
				h.broadcastUserList()
			}

		case messageBytes := <-h.broadcast:
			// 反序列化，检查消息类型并存入数据库
			var msg Message
			if err := json.Unmarshal(messageBytes, &msg); err != nil {
				// 如果 JSON 解析失败，则忽略这条消息
				continue
			}

			if msg.Type == "private" && msg.To != "" {
				// --- 处理私聊消息 ---
				var targetClient *Client
				// 遍历在线用户，找到目标客户端
				for client := range h.clients {
					if client.name == msg.To {
						targetClient = client
						break
					}
				}
				// 如果找到了目标客户端，则只向他发送
				if targetClient != nil {
					// 找到发送者客户端，以便也给自己发一份（已读回执）
					var senderClient *Client
					for client := range h.clients {
						if client.name == msg.Sender {
							senderClient = client
							break
						}
					}

					// 发送给目标
					select {
					case targetClient.send <- messageBytes:
					default:
						close(targetClient.send)
						delete(h.clients, targetClient)
					}
					// 如果发送者存在，也给自己发一份
					if senderClient != nil {
						select {
						case senderClient.send <- messageBytes:
						default:
							close(senderClient.send)
							delete(h.clients, senderClient)
						}
					}
				}
			} else {
				// --- 处理广播消息 (包括 system 和 broadcast) ---
				if msg.Type == "broadcast" {
					// 只有广播消息才存入数据库
					chatMsg := ChatMessage{
						Sender:  msg.Sender,
						Content: msg.Content,
					}
					DB.Create(&chatMsg)
					msg.Timestamp = chatMsg.CreatedAt
					messageBytes, _ = json.Marshal(msg)
				}
				// 正常广播
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
