// user.go
package main

import (
	"gorm.io/driver/mysql" // 导入 MySQL 驱动
	"gorm.io/gorm"
)

var DB *gorm.DB

// User 定义了用户模型
type User struct {
	gorm.Model
	Username string `gorm:"unique"`
	Password string
	IsAdmin  bool `gorm:"default:false"` // 新增：是否为管理员
}

// InitDB 初始化数据库连接
func InitDB() {
	var err error
	// DSN (Data Source Name) for MySQL connection
	// 格式: "user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
	// 请将 "your_password" 替换成你自己的 MySQL root 密码
	dsn := "root:123456@tcp(127.0.0.1:3306)/go_chat?charset=utf8mb4&parseTime=True&loc=Local"

	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	// 自动迁移 User 和 ChatMessage 模型
	DB.AutoMigrate(&User{}, &ChatMessage{})
}
