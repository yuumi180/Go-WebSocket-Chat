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
	"time"

	"github.com/rs/cors"
)

type AuthPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// createIndexIfNotExists 辅助函数，用于在 MySQL 中创建索引（如果不存在）
func createIndexIfNotExists(tableName, indexName, columns string) {
	// 检查索引是否已存在
	var count int64
	query := `
		SELECT COUNT(*) 
		FROM information_schema.STATISTICS 
		WHERE table_schema = DATABASE() 
		AND table_name = ? 
		AND index_name = ?`
	DB.Raw(query, tableName, indexName).Scan(&count)
	
	if count == 0 {
		// 索引不存在，创建它
		createSQL := fmt.Sprintf("CREATE INDEX %s ON %s (%s)", indexName, tableName, columns)
		result := DB.Exec(createSQL)
		if result.Error != nil {
			log.Printf("创建索引失败：%v", result.Error)
		} else {
			log.Printf("成功创建索引：%s", indexName)
		}
	} else {
		log.Printf("索引已存在：%s", indexName)
	}
}

// createCompositeIndexIfExists 创建优化的复合索引
func createCompositeIndexIfExists() {
	// 只为 receiver 和 is_read 创建复合索引，这两个字段足够优化离线消息查询
	createIndexIfNotExists("chat_messages", "idx_receiver_read", "receiver(50), is_read")
}

// createDefaultAdmin 创建默认管理员账号
func createDefaultAdmin() {
	var admin User
	result := DB.Where("username = ?", "admin").First(&admin)
	if result.Error != nil {
		// 管理员不存在，创建默认管理员
		admin = User{
			Username: "admin",
			Password: "admin123", // 默认密码
			IsAdmin:  true,
		}
		DB.Create(&admin)
		log.Println("默认管理员账号已创建：用户名=admin, 密码=admin123")
	} else {
		log.Println("管理员账号已存在")
	}
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

	// 将生成的 token 以 JSON 格式返回给前端，包含 isAdmin 信息
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":   tokenString,
		"isAdmin": user.IsAdmin,
	})
}

// handleGetUsers 获取所有用户列表（仅管理员可用）
func handleGetUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 验证 JWT token
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "Token is required", http.StatusUnauthorized)
		return
	}

	username, err := ValidateJWT(tokenStr)
	if err != nil {
		log.Printf("Token 验证失败：%v", err)
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	log.Printf("管理员 %s 请求用户列表", username)

	// 检查是否是管理员
	var currentUser User
	result := DB.Where("username = ?", username).First(&currentUser)
	if result.Error != nil {
		log.Printf("查找用户失败：%v", result.Error)
		http.Error(w, "User not found", http.StatusForbidden)
		return
	}
	
	if !currentUser.IsAdmin {
		log.Printf("用户 %s 不是管理员", username)
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}

	// 获取所有用户
	var users []User
	DB.Find(&users)

	// 构建用户列表
	type UserInfo struct {
		ID        uint      `json:"id"`
		Username  string    `json:"username"`
		IsAdmin   bool      `json:"isAdmin"`
		CreatedAt time.Time `json:"createdAt"`
	}

	var userList []UserInfo
	for _, user := range users {
		userList = append(userList, UserInfo{
			ID:        user.ID,
			Username:  user.Username,
			IsAdmin:   user.IsAdmin,
			CreatedAt: user.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userList)
}

// handleDeleteUser 处理删除用户的请求（仅管理员可用）
func handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 验证 JWT token
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

	// 检查是否是管理员
	var currentUser User
	result := DB.Where("username = ?", username).First(&currentUser)
	if result.Error != nil || !currentUser.IsAdmin {
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}

	// 解析要删除的用户名
	var req struct {
		TargetUsername string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if req.TargetUsername == "" {
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}

	// 不能删除自己
	if req.TargetUsername == username {
		http.Error(w, "Cannot delete yourself", http.StatusBadRequest)
		return
	}

	// 删除用户及其消息
	var targetUser User
	deleteResult := DB.Where("username = ?", req.TargetUsername).First(&targetUser)
	if deleteResult.Error != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// 删除该用户的所有消息
	DB.Where("sender = ? OR receiver = ?", req.TargetUsername, req.TargetUsername).Delete(&ChatMessage{})

	// 删除用户
	DB.Delete(&targetUser)

	log.Printf("管理员 %s 删除了用户 %s", username, req.TargetUsername)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "User deleted successfully")
}

func main() {
	InitDB()
	
	// GORM 会自动通过 struct tag 创建索引，不需要手动创建
	// 如果需要优化，可以在这里添加额外的复合索引
	createCompositeIndexIfExists()
	
	// 创建默认管理员账号（如果不存在）
	createDefaultAdmin()

	hub := newHub()
	go hub.run()

	// 创建一个 HTTP multiplexer (路由分发器)
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	mux.HandleFunc("/register", handleRegister)
	mux.HandleFunc("/login", handleLogin)
	mux.HandleFunc("/admin/users", handleGetUsers)
	mux.HandleFunc("/admin/delete-user", handleDeleteUser)
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

	// --- 优雅关闭的逻辑保持不变 ---
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	fmt.Println("\nShutting down server...")
}
