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
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	name string
}

// --- 这是核心修正 ---
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
		// 'message' 是从客户端收到的原始 JSON 字节流
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		// 直接将从客户端收到的原始消息字节流转发给 Hub
		// Hub 将负责解析、处理和分发
		c.hub.broadcast <- message
	}
}

// writePump 函数保持不变
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
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs 函数保持不变
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request, username string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256), name: username}
	client.hub.register <- client

	go func() {
		var messages []ChatMessage
		DB.Order("created_at desc").Limit(50).Find(&messages)
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}
		for _, msg := range messages {
			historyMsg := Message{
				Type:      "broadcast",
				Sender:    msg.Sender,
				Content:   msg.Content,
				Timestamp: msg.CreatedAt,
			}
			msgBytes, _ := json.Marshal(historyMsg)
			client.send <- msgBytes
		}
	}()

	go client.writePump()
	go client.readPump()
}
