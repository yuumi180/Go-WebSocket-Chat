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

	// 对密码进行哈希处理
	hashedPassword, err := HashPassword(payload.Password)
	if err != nil {
		http.Error(w, "Failed to process password", http.StatusInternalServerError)
		return
	}

	user := User{Username: payload.Username, Password: hashedPassword}

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

	log.Printf("登录尝试 - 用户名：%s", payload.Username)

	var user User
	// 在数据库中查找匹配的用户名
	result := DB.Where("username = ?", payload.Username).First(&user)
	if result.Error != nil {
		log.Printf("用户不存在：%s, 错误：%v", payload.Username, result.Error)
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	log.Printf("找到用户 - ID: %d, 用户名：%s, 密码哈希长度：%d", user.ID, user.Username, len(user.Password))

	// 验证密码
	passwordValid := CheckPasswordHash(payload.Password, user.Password)
	log.Printf("密码验证结果：%v", passwordValid)
	
	if !passwordValid {
		log.Printf("密码验证失败 - 输入密码长度：%d", len(payload.Password))
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	log.Printf("登录成功 - 用户名：%s, isAdmin: %v", user.Username, user.IsAdmin)

	// 验证通过，为该用户生成一个 JWT
	tokenString, err := GenerateJWT(user.Username)
	if err != nil {
		log.Printf("生成 token 失败：%v", err)
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

	log.Printf("准备删除用户 - ID: %d, 用户名：%s", targetUser.ID, targetUser.Username)

	// 先删除该用户的所有消息（外键关联）
	messagesDeleted := DB.Where("sender = ? OR receiver = ?", req.TargetUsername, req.TargetUsername).Delete(&ChatMessage{})
	log.Printf("删除了 %d 条消息", messagesDeleted.RowsAffected)

	// 使用 Unscoped 强制物理删除，忽略软删除
	// 直接使用 Exec 执行原生 SQL 删除
	rawDelete := DB.Exec("DELETE FROM users WHERE username = ?", req.TargetUsername)
	log.Printf("物理删除用户结果 - 影响行数：%d, 错误：%v", rawDelete.RowsAffected, rawDelete.Error)
	
	if rawDelete.Error != nil {
		log.Printf("物理删除用户失败：%v", rawDelete.Error)
		http.Error(w, "Failed to delete user: "+rawDelete.Error.Error(), http.StatusInternalServerError)
		return
	}
	
	if rawDelete.RowsAffected == 0 {
		log.Printf("警告：没有删除任何用户记录")
		http.Error(w, "No user deleted", http.StatusInternalServerError)
		return
	}

	log.Printf("管理员 %s 删除了用户 %s (ID: %d)", username, req.TargetUsername, targetUser.ID)
	
	// 新增：关闭被删除用户的 WebSocket 连接
	closeUserConnection(req.TargetUsername)
	
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "User deleted successfully")
}

// closeUserConnection 关闭指定用户的 WebSocket 连接
func closeUserConnection(targetUsername string) {
	// 获取 Hub 实例（需要从全局变量或者参数传递）
	// 由于 hub 是在 main 函数中创建的，我们需要通过全局变量或者修改函数签名来访问
	// 这里我们通过修改 client.go 中的 Client 结构来实现
	log.Printf("准备关闭用户 %s 的 WebSocket 连接", targetUsername)
	// 具体的关闭逻辑需要在 client.go 和 hub.go 中配合实现
}

func main() {
	InitDB()
	
	// 迁移旧密码（如果有）
	MigratePasswords()
	
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
	mux.HandleFunc("/admin/delete-user", func(w http.ResponseWriter, r *http.Request) {
		// 使用闭包传递 hub
		handleDeleteUserWithHub(hub, w, r)
	})
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
	// 配置 CORS 中间件 - 放宽限制以支持本地开发
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // 允许所有来源（本地开发用，生产环境改为实际域名）
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           86400, // 缓存预检请求结果 24 小时
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

// handleDeleteUserWithHub 处理删除用户的请求（带 Hub 参数）
func handleDeleteUserWithHub(hub *Hub, w http.ResponseWriter, r *http.Request) {
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

	log.Printf("准备删除用户 - ID: %d, 用户名：%s", targetUser.ID, targetUser.Username)

	// 先删除该用户的所有消息（外键关联）
	messagesDeleted := DB.Where("sender = ? OR receiver = ?", req.TargetUsername, req.TargetUsername).Delete(&ChatMessage{})
	log.Printf("删除了 %d 条消息", messagesDeleted.RowsAffected)

	// 使用 Unscoped 强制物理删除，忽略软删除
	// 直接使用 Exec 执行原生 SQL 删除
	rawDelete := DB.Exec("DELETE FROM users WHERE username = ?", req.TargetUsername)
	log.Printf("物理删除用户结果 - 影响行数：%d, 错误：%v", rawDelete.RowsAffected, rawDelete.Error)
	
	if rawDelete.Error != nil {
		log.Printf("物理删除用户失败：%v", rawDelete.Error)
		http.Error(w, "Failed to delete user: "+rawDelete.Error.Error(), http.StatusInternalServerError)
		return
	}
	
	if rawDelete.RowsAffected == 0 {
		log.Printf("警告：没有删除任何用户记录")
		http.Error(w, "No user deleted", http.StatusInternalServerError)
		return
	}

	log.Printf("管理员 %s 删除了用户 %s (ID: %d)", username, req.TargetUsername, targetUser.ID)
	
	// 新增：关闭被删除用户的 WebSocket 连接
	hub.DisconnectUser(req.TargetUsername)
	
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "User deleted successfully")
}
