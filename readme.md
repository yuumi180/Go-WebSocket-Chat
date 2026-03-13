# WebSocket Chat - Go 语言实现的实时聊天系统

一个基于 Go 语言和 WebSocket 技术构建的实时聊天系统，支持群聊、私聊、用户管理等功能，具有高性能和高并发特性。

## 📋 项目特点

- **实时通信**：基于 WebSocket 实现全双工通信，消息即时送达
- **多种聊天模式**：支持群聊和一对一私聊
- **离线消息**：自动存储离线消息，用户上线后自动推送
- **用户认证**：基于 JWT 的身份验证系统
- **管理员功能**：用户管理、删除用户等后台管理功能
- **高性能架构**：
    - 消息队列批量处理
    - 对象池复用
    - 数据库连接池优化
    - 复合索引优化查询
- **安全性**：
    - 密码 bcrypt 加密
    - CORS 跨域支持
    - 速率限制中间件
- **完善的日志系统**：分级日志记录（INFO、WARN、ERROR）

## 🛠️ 技术栈

### 后端
- **语言**: Go 1.25.0+
- **WebSocket**: gorilla/websocket
- **ORM**: GORM v1.31.1
- **数据库**: MySQL
- **JWT**: golang-jwt/jwt/v5
- **CORS**: rs/cors
- **限流**: golang.org/x/time/rate
- **加密**: golang.org/x/crypto/bcrypt

### 前端
- 原生 JavaScript + HTML5 + CSS3
- WebSocket API
- LocalStorage 本地存储
- 响应式设计（支持移动端）

## 📦 安装与运行

### 环境要求

- Go 1.25.0 或更高版本
- MySQL 5.7+ 或 MySQL 8.0+
- Git

### 1. 克隆项目

bash 
git clone <项目地址> 
cd websocket-chat

### 2. 安装依赖
bash 
go mod download

### 3. 配置数据库

编辑 `config/config.json` 文件，修改数据库连接信息：

### 4. 创建数据库

在 MySQL 中创建数据库：

sql 

CREATE DATABASE go_chat CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

### 5. 运行程序

bash 
go run .

启动成功后会看到以下提示：

Chat server started on :8080 
Open http://localhost:8080 in your browser. 
默认管理员账号已创建：用户名=admin, 密码=admin123

**### 6. 访问聊天室

打开浏览器访问：http://localhost:8080

## 🎯 使用指南

### 普通用户功能

#### 1. 注册账号
- 点击登录页面的"立即注册"链接
- 输入用户名（至少 3 个字符）和密码（至少 6 个字符）
- 点击"注册"按钮

#### 2. 登录
- 输入用户名和密码
- 点击"登录"按钮
- 登录成功后自动进入聊天室

#### 3. 群聊
- 登录后默认进入"群聊大厅"
- 在输入框输入消息，点击"发送"或按 Enter 键
- 可以发送文本消息和图片
- 所有在线用户都能看到群聊消息

#### 4. 私聊
- 在左侧用户列表中点击要私聊的用户
- 切换到私聊模式
- 输入消息并发送
- 如果对方不在线，消息会自动保存，等对方上线后推送

#### 5. 查看历史消息
- 进入群聊时会自动加载最近 50 条群聊消息
- 私聊消息会在数据库中保存，重新进入时可以查看

### 管理员功能

默认管理员账号：
- **用户名**: admin
- **密码**: admin123

⚠️ **首次登录后请立即修改密码！**

#### 管理员特权

1. **访问用户管理面板**
    - 点击左侧的"👑 用户管理"按钮
    - 查看所有用户列表

2. **查看用户信息**
    - 用户 ID、用户名
    - 角色（管理员/普通用户）
    - 注册时间
    - 在线状态（实时刷新）

3. **删除用户**
    - 点击用户右侧的"删除"按钮
    - 确认后删除该用户及其所有消息记录
    - ⚠️ 无法删除其他管理员
    - ⚠️ 无法删除自己

## 🔌 API 接口

### 认证接口

#### 注册

http POST /register Content-Type: application/json
{ "username": "testuser", "password": "testpass" }

响应：
- 201: 注册成功
- 400: 请求参数错误
- 409: 用户名已存在

#### 登录

http POST /login Content-Type: application/json
{ "username": "testuser", "password": "testpass" }

响应：

json { "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...", "isAdmin": false }

### 管理员接口

#### 获取用户列表

http GET /admin/users?token=<JWT_TOKEN>

#### 删除用户

http POST /admin/delete-user?token=<JWT_TOKEN> Content-Type: application/json
{ "username": "targetuser" }

### WebSocket 连接

javascript const ws = new WebSocket(ws://localhost:8080/ws?token=${token});

#### 消息类型

**发送消息格式：**

json { "type": "broadcast", // 或 "private" "sender": "username", "to": "receiver", // 私聊时需要指定 "content": "消息内容", "contentType": "text" // 或 "image" }

**接收消息类型：**
- `broadcast`: 群聊消息
- `private`: 私聊消息
- `system`: 系统消息
- `user_list`: 用户列表更新
- `typing`: 正在输入
- `stop_typing`: 停止输入

## 🧪 测试

### 运行单元测试

bash 

go test -v

### 运行性能测试

bash 

go test -bench=. -benchmem

### 基准测试

项目内置了性能基准测试，可以测试不同负载下的性能表现：

bash
轻负载测试（50 用户，每人 20 条消息）
go test -run=TestLightLoad -v
正常负载测试（100 用户，每人 50 条消息）
go test -run=TestConcurrentConnections -v
高负载测试（200 用户，每人 100 条消息）
go test -run=TestHighLoad -v

## 📊 性能指标

根据基准测试结果：

| 测试场景 | 并发用户 | 消息数/用户 | 吞吐率      | 成功率 |
| -------- | -------- | ----------- | ----------- | ------ |
| 轻负载   | 50       | 20          | >5000 条/秒 | >99%   |
| 正常负载 | 100      | 50          | >2000 条/秒 | >95%   |
| 高负载   | 200      | 100         | >1000 条/秒 | >90%   |

## 🔒 安全特性

1. **密码加密**：使用 bcrypt 进行密码哈希处理
2. **JWT 认证**：Token 有效期 24 小时
3. **CORS 保护**：可配置允许的源
4. **速率限制**：防止恶意请求（基于 IP）
5. **权限控制**：管理员功能需要相应权限
6. **XSS 防护**：前端对用户输入进行转义

## 🎨 前端特性

- **响应式设计**：支持桌面端和移动端
- **实时在线状态**：显示用户在线/离线状态
- **未读消息提醒**：显示未读消息数量徽章
- **输入状态提示**：显示"正在输入..."状态
- **图片上传**：支持发送图片消息
- **浏览器通知**：新消息桌面通知
- **优雅的重连机制**：网络断开自动重连

## 🐛 故障排除

### 常见问题

#### 1. 数据库连接失败

panic: failed to connect database

**解决方法**：
- 检查 MySQL 服务是否启动
- 确认 `config/config.json` 中的数据库配置正确
- 确保数据库 `go_chat` 已创建

#### 2. Token 验证失败

Invalid or expired token

**解决方法**：
- Token 可能已过期，请重新登录
- 清除浏览器缓存和 localStorage

#### 3. WebSocket 连接失败
**解决方法**：
- 检查服务器是否正常运行
- 确认防火墙没有阻止 8080 端口
- 检查 Token 是否有效

#### 4. 密码迁移提示

密码迁移完成！

**说明**：这是正常现象，系统会自动将旧版本的明文密码升级为 bcrypt 加密

---

**注意**：生产环境部署时，请务必：
1. 修改 JWT 密钥为强随机字符串
2. 修改默认管理员密码
3. 配置合适的 CORS 策略（不要使用通配符）
4. 启用 HTTPS
5. 配置数据库备份策略
