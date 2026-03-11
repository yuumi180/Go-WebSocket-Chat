// hub.go
package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

// Hub 添加消息队列
type Hub struct {
	mu            sync.RWMutex
	clients       map[*Client]bool
	broadcast     chan []byte
	register      chan *Client
	unregister    chan *Client
	messageQueue  chan *ChatMessage // 消息队列，用于批量写入
}

func newHub() *Hub {
	hub := &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		messageQueue: make(chan *ChatMessage, 1000), // 队列容量 1000
	}
	
	// 启动异步写入协程
	go hub.batchSaveMessages()
	
	return hub
}

// batchSaveMessages 批量保存消息到数据库
func (h *Hub) batchSaveMessages() {
	ticker := time.NewTicker(100 * time.Millisecond) // 每 100ms 批量写入一次
	defer ticker.Stop()

	batch := make([]*ChatMessage, 0, 100)
	
	for {
		select {
		case msg := <-h.messageQueue:
			batch = append(batch, msg)
			if len(batch) >= 100 {
				// 达到批量大小，立即写入
				DB.CreateInBatches(batch, 100)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				// 定时写入
				DB.CreateInBatches(batch, 100)
				batch = batch[:0]
			}
		}
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

// 添加消息对象池
var messagePool = sync.Pool{
	New: func() interface{} {
		return &Message{}
	},
}

// run 方法中使用对象池
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
			msg := messagePool.Get().(*Message)
			if err := json.Unmarshal(messageBytes, msg); err != nil {
				messagePool.Put(msg)
				continue
			}

			// 优化：放入队列而不是直接写入
			if msg.Type == "broadcast" || msg.Type == "private" {
				chatMsg := &ChatMessage{
					Sender:   msg.Sender,
					Receiver: msg.To,
					Type:     msg.Type,
					Content:  msg.Content,
				}
				
				// 非阻塞放入队列
				select {
				case h.messageQueue <- chatMsg:
				default:
					// 队列已满，直接写入（降级处理）
					DB.Create(chatMsg)
				}

				// 使用当前时间戳（不等待数据库）
				msg.Timestamp = time.Now()
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
				// 优化 2：批量发送，减少锁竞争
				h.broadcastToAllOptimized(messageBytes)
			}

			// 使用完后归还对象池
			messagePool.Put(msg)
		}
	}
}

// broadcastToAllOptimized 优化的批量发送版本
func (h *Hub) broadcastToAllOptimized(msgBytes []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// 预分配客户端列表
	clients := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}

	// 批量发送
	for _, client := range clients {
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
