package main

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 测试配置
type BenchmarkConfig struct {
	ConcurrentUsers int           // 并发用户数
	MessagesPerUser int           // 每个用户发送的消息数
	MessageDelay    time.Duration // 消息发送间隔
}

// 默认配置 - 更接近真实场景
var defaultConfig = BenchmarkConfig{
	ConcurrentUsers: 100,
	MessagesPerUser: 50,
	MessageDelay:    0, // 移除延迟，测试极限性能
}

// TestConcurrentConnections 测试并发连接性能
func TestConcurrentConnections(t *testing.T) {
	runBenchmark(defaultConfig)
}

// TestHighLoad 测试高负载情况
func TestHighLoad(t *testing.T) {
	config := BenchmarkConfig{
		ConcurrentUsers: 2000, // 增加到 1000 用户
		MessagesPerUser: 100,
		MessageDelay:    0, // 无延迟
	}
	runBenchmark(config)
}

// TestLightLoad 测试轻负载情况
func TestLightLoad(t *testing.T) {
	config := BenchmarkConfig{
		ConcurrentUsers: 50,
		MessagesPerUser: 20,
		MessageDelay:    0,
	}
	runBenchmark(config)
}

func runBenchmark(config BenchmarkConfig) {
	fmt.Printf("\n========== 开始性能测试 ==========\n")
	fmt.Printf("并发用户数：%d\n", config.ConcurrentUsers)
	fmt.Printf("每用户消息数：%d\n", config.MessagesPerUser)
	fmt.Printf("消息延迟：%v\n", config.MessageDelay)
	fmt.Printf("====================================\n\n")

	// 初始化数据库（测试环境）
	InitDB()

	// 创建测试 Hub
	hub := newHub()
	go hub.run()

	// 等待 Hub 启动
	time.Sleep(100 * time.Millisecond)

	var (
		wg             sync.WaitGroup
		totalSent      int64 = 0
		totalErrors    int64 = 0
		startTime            = time.Now()
		lastSecondSent int64 = 0
	)

	// 启动性能监控
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			currentSent := atomic.LoadInt64(&totalSent)
			currentErrors := atomic.LoadInt64(&totalErrors)

			sentPerSecond := currentSent - lastSecondSent
			lastSecondSent = currentSent

			elapsed := time.Since(startTime).Seconds()
			if elapsed > 0 {
				fmt.Printf("\r[进度] 已发送：%d | 错误：%d | 速率：%d 条/秒 | Goroutines: %d",
					currentSent, currentErrors, sentPerSecond, getGoroutineCount())
			}
		}
	}()

	// 创建并发用户
	for i := 0; i < config.ConcurrentUsers; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()

			// 创建模拟客户端
			client := &Client{
				hub:  hub,
				name: fmt.Sprintf("user%d", userID),
				send: make(chan []byte, 256),
			}

			// 注册到 Hub
			hub.register <- client

			// 启动客户端读取协程（仅消费消息，不统计）
			go func() {
				for range client.send {
					// 消费消息，但不计数
				}
			}()

			// 发送消息
			for j := 0; j < config.MessagesPerUser; j++ {
				msg := Message{
					Type:      "broadcast",
					Sender:    fmt.Sprintf("user%d", userID),
					Content:   fmt.Sprintf("消息 %d 来自用户 %d", j, userID),
					Timestamp: time.Now(),
				}

				msgBytes, err := json.Marshal(msg)
				if err != nil {
					atomic.AddInt64(&totalErrors, 1)
					continue
				}

				hub.broadcast <- msgBytes
				atomic.AddInt64(&totalSent, 1)

				// 模拟真实用户的输入延迟
				time.Sleep(config.MessageDelay)
			}

			// 断开连接
			hub.unregister <- client
		}(i)
	}

	// 等待所有用户完成
	wg.Wait()

	// 等待 Hub 处理完成
	time.Sleep(500 * time.Millisecond)

	elapsed := time.Since(startTime)
	totalMessages := config.ConcurrentUsers * config.MessagesPerUser
	messagesPerSecond := float64(totalSent) / elapsed.Seconds()
	successRate := float64(totalSent-totalErrors) / float64(totalSent) * 100

	fmt.Printf("\n\n========== 性能测试结果 ==========\n")
	fmt.Printf("并发用户数：%d\n", config.ConcurrentUsers)
	fmt.Printf("每用户消息数：%d\n", config.MessagesPerUser)
	fmt.Printf("计划发送：%d 条\n", totalMessages)
	fmt.Printf("实际发送：%d 条\n", totalSent)
	fmt.Printf("错误数量：%d 条\n", totalErrors)
	fmt.Printf("总耗时：%v\n", elapsed)
	fmt.Printf("消息吞吐率：%.2f 条/秒\n", messagesPerSecond)
	fmt.Printf("成功率：%.2f%%\n", successRate)
	fmt.Printf("最终 Goroutines: %d\n", getGoroutineCount())
	fmt.Printf("====================================\n")

	// 性能评级
	if messagesPerSecond > 5000 && successRate > 99 {
		fmt.Printf("\n🌟 性能评级：优秀 (Excellent)\n")
	} else if messagesPerSecond > 2000 && successRate > 95 {
		fmt.Printf("\n✅ 性能评级：良好 (Good)\n")
	} else if messagesPerSecond > 1000 && successRate > 90 {
		fmt.Printf("\n⚠️  性能评级：一般 (Fair)\n")
	} else {
		fmt.Printf("\n❌ 性能评级：需要优化 (Needs Improvement)\n")
	}
}

// TestHubBroadcastPerformance 测试 Hub 广播性能
func TestHubBroadcastPerformance(t *testing.T) {
	hub := newHub()
	go hub.run()

	// 创建模拟客户端
	clientCount := 100
	clients := make([]*Client, 0, clientCount)

	for i := 0; i < clientCount; i++ {
		client := &Client{
			hub:  hub,
			name: fmt.Sprintf("user%d", i),
			send: make(chan []byte, 256),
		}
		hub.register <- client
		clients = append(clients, client)
	}

	// 等待所有客户端注册完成
	time.Sleep(100 * time.Millisecond)

	// 测试广播性能
	message := []byte(`{"type":"broadcast","content":"性能测试消息","sender":"benchmark"}`)

	startTime := time.Now()
	hub.broadcastToAll(message)
	elapsed := time.Since(startTime)

	fmt.Printf("\n========== 广播性能测试 ==========\n")
	fmt.Printf("客户端数量：%d\n", len(clients))
	fmt.Printf("广播耗时：%v\n", elapsed)
	fmt.Printf("单条消息平均耗时：%v\n", elapsed/time.Duration(clientCount))
	fmt.Printf("====================================\n")

	// 清理
	for _, client := range clients {
		hub.unregister <- client
	}
}

// TestOfflineMessagePerformance 测试离线消息性能
func TestOfflineMessagePerformance(t *testing.T) {
	// 初始化数据库（如果需要）
	InitDB()

	hub := newHub()
	go hub.run()

	// 模拟用户下线
	user1 := &Client{
		hub:  hub,
		name: "offline_user1",
		send: make(chan []byte, 256),
	}

	// 注册然后立即注销（模拟下线）
	hub.register <- user1
	time.Sleep(50 * time.Millisecond)
	hub.unregister <- user1

	// 模拟其他用户发送离线消息
	sender := &Client{
		hub:  hub,
		name: "online_user",
		send: make(chan []byte, 256),
	}
	hub.register <- sender

	messageCount := 100
	startTime := time.Now()

	for i := 0; i < messageCount; i++ {
		msg := Message{
			Type:      "private",
			Sender:    "online_user",
			To:        "offline_user1",
			Content:   fmt.Sprintf("离线消息 %d", i),
			Timestamp: time.Now(),
		}
		msgBytes, _ := json.Marshal(msg)
		hub.broadcast <- msgBytes
	}

	elapsed := time.Since(startTime)

	fmt.Printf("\n========== 离线消息性能测试 ==========\n")
	fmt.Printf("发送离线消息数：%d\n", messageCount)
	fmt.Printf("总耗时：%v\n", elapsed)
	fmt.Printf("平均每条消息：%v\n", elapsed/time.Duration(messageCount))
	fmt.Printf("========================================\n")

	hub.unregister <- sender
}

// 辅助函数：获取 Goroutine 数量
func getGoroutineCount() int {
	return runtime.NumGoroutine()
}

// BenchmarkWebSocket 基准测试
func BenchmarkWebSocket(b *testing.B) {
	InitDB()

	hub := newHub()
	go hub.run()

	client := &Client{
		hub:  hub,
		name: "benchmark_user",
		send: make(chan []byte, 256),
	}
	hub.register <- client

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := Message{
			Type:      "broadcast",
			Sender:    "benchmark_user",
			Content:   fmt.Sprintf("基准测试消息 %d", i),
			Timestamp: time.Now(),
		}
		msgBytes, _ := json.Marshal(msg)
		hub.broadcast <- msgBytes
	}

	hub.unregister <- client
}

// BenchmarkHubBroadcast 基准测试广播性能
func BenchmarkHubBroadcast(b *testing.B) {
	InitDB()

	hub := newHub()
	go hub.run()

	// 创建 100 个客户端
	for i := 0; i < 100; i++ {
		client := &Client{
			hub:  hub,
			name: fmt.Sprintf("user%d", i),
			send: make(chan []byte, 256),
		}
		hub.register <- client
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := []byte(`{"type":"broadcast","content":"benchmark"}`)
		hub.broadcastToAll(msg)
	}
}
