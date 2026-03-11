// main.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/cors"
)

type AuthPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleRegister 处理用户注册请求。
func handleRegister(w http.ResponseWriter, r *http.Request) {
	// 只接受 POST 请求
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload AuthPayload
	// 解析请求体中的 JSON 数据
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// 检查用户名和密码是否为空
	if payload.Username == "" || payload.Password == "" {
		http.Error(w, "Username and password cannot be empty", http.StatusBadRequest)
		return
	}

	// 在生产环境中，密码应该使用 bcrypt 等算法进行哈希处理！
	// 这里为了教程的简洁性，直接存储明文密码。
	user := User{Username: payload.Username, Password: payload.Password}

	// 创建用户记录
	result := DB.Create(&user)
	if result.Error != nil {
		// 如果出错，很可能是因为用户名已存在 (因为我们设置了 unique 索引)
		http.Error(w, "Username already exists", http.StatusConflict)
		return
	}

	// 返回 201 Created 状态码，表示资源创建成功
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintln(w, "User registered successfully")
}

// handleLogin 处理用户登录请求。
func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload AuthPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	var user User
	// 在数据库中查找匹配的用户名和密码
	result := DB.Where("username = ? AND password = ?", payload.Username, payload.Password).First(&user)
	if result.Error != nil {
		// 如果找不到记录，返回 401 Unauthorized
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	// 验证通过，为该用户生成一个 JWT
	tokenString, err := GenerateJWT(user.Username)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// 将生成的 token 以 JSON 格式返回给前端
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
}

func main() {
	InitDB()

	hub := newHub()
	go hub.run()

	// 创建一个 HTTP multiplexer (路由分发器)
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	mux.HandleFunc("/register", handleRegister)
	mux.HandleFunc("/login", handleLogin)
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.URL.Query().Get("token")
		if tokenStr == "" {
			http.Error(w, "Token is required", http.StatusUnauthorized)
			return
		}
		username, err := ValidateJWT(tokenStr)
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}
		serveWs(hub, w, r, username)
	})

	// --- 核心改动在这里 ---
	// 配置 CORS 中间件
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // 允许所有来源，在生产环境中应设置为你的前端域名
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	// 使用 CORS 中间件包裹我们的路由分发器
	handler := c.Handler(mux)

	// 在一个独立的 goroutine 中启动 HTTP 服务器，并使用包裹后的 handler
	go func() {
		fmt.Println("Chat server started on :8080")
		fmt.Println("Open http://localhost:8080 in your browser.")
		if err := http.ListenAndServe(":8080", handler); err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	}()

	// ... (优雅关闭的逻辑保持不变)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	fmt.Println("\nShutting down server...")
}
