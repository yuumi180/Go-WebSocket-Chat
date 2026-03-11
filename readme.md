# WebSocket 聊天室 - 项目说明

## 技术栈
- 后端：Go + Gorilla WebSocket + GORM
- 前端：原生 JavaScript + HTML5 + CSS3
- 数据库：MySQL 8.0
- 认证：JWT

## 核心功能
1. 实时聊天（群聊/私聊）
2. 离线消息推送
3. 管理员系统
4. 用户管理
5. 浏览器通知
6. 未读消息计数
7. 密码 bcrypt 加密
8. 自动重连机制

## 性能优化
- 数据库连接池
- 索引优化
- WebSocket 连接池管理
- 消息批量推送

## 安全特性
- JWT 认证
- bcrypt 密码加密
- XSS 防护
- CORS 配置