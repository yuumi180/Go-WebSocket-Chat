// client.go
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// 允许的写入等待时间。
	writeWait = 10 * time.Second
	// 允许的读取下一次 pong 消息的等待时间。
	pongWait = 60 * time.Second
	// 发送 ping 到 peer 的周期。必须小于 pongWait。
	pingPeriod = (pongWait * 9) / 10
	// 允许从 peer 读取的最大消息大小。
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有跨域请求
	},
}

// Client 是 WebSocket 客户端和 Hub 之间的中间人。
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	name string
}

// readPump 将消息从 WebSocket 连接泵送到 Hub。
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { _ = c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	// 广播用户加入的消息
	joinMsg := Message{
		Type:      "system",
		Sender:    "System",
		Content:   c.name + " joined the chat.",
		Timestamp: time.Now(),
	}
	msgBytes, _ := json.Marshal(joinMsg)
	c.hub.broadcast <- msgBytes

	// 循环读取来自客户端的聊天消息
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		// 将收到的普通聊天消息封装成 Message 结构体
		broadcastMsg := Message{
			Type:    "broadcast",
			Sender:  c.name,
			Content: string(message),
			// Timestamp 将在 Hub 中被添加
		}
		broadcastMsgBytes, _ := json.Marshal(broadcastMsg)

		// 放入 hub 的广播 channel
		c.hub.broadcast <- broadcastMsgBytes
	}
}

// writePump 将消息从 Hub 泵送到 WebSocket 连接。
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub 关闭了这个 channel。
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// 将消息写入 WebSocket 连接。
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			// 定期发送 ping 消息以保持连接活跃。
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs 处理来自 peer 的 websocket 请求。
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request, username string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256), name: username}
	client.hub.register <- client

	// 在一个独立的 goroutine 中加载并发送历史消息
	go func() {
		var messages []ChatMessage
		// 查询数据库，获取最近的50条消息，按创建时间倒序
		DB.Order("created_at desc").Limit(50).Find(&messages)

		// 反转消息顺序，使其按时间正序发送
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}

		for _, msg := range messages {
			historyMsg := Message{
				Type:      "broadcast",
				Sender:    msg.Sender,
				Content:   msg.Content,
				Timestamp: msg.CreatedAt, // 使用数据库中的时间
			}
			msgBytes, _ := json.Marshal(historyMsg)
			// 直接发送到客户端的 send channel，不通过 hub 广播
			client.send <- msgBytes
		}
	}()

	// 启动并发的读写 goroutine
	go client.writePump()
	go client.readPump()
}
