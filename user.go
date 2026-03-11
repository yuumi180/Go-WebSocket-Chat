// user.go
package main

import (
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

// User 定义了用户模型
type User struct {
	gorm.Model
	Username string `gorm:"unique"`
	Password string
	IsAdmin  bool `gorm:"default:false"`
}

// HashPassword 使用 bcrypt 对密码进行哈希处理
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash 验证密码是否匹配
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// InitDB 初始化数据库连接
func InitDB() {
	var err error
	dsn := "root:123456@tcp(127.0.0.1:3306)/go_chat?charset=utf8mb4&parseTime=True&loc=Local"

	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// 获取底层 sql.DB 并配置连接池
	sqlDB, err := DB.DB()
	if err != nil {
		panic("failed to get database instance")
	}

	// 设置连接池参数
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	DB.AutoMigrate(&User{}, &ChatMessage{})
	createOptimizedIndexes()
}

// createOptimizedIndexes 创建优化的索引（修复版本）
func createOptimizedIndexes() {
	// 检查并创建复合索引：加速离线消息查询
	createIndexIfNotExistsMySQL("chat_messages", "idx_receiver_type_read", "receiver, type, is_read")
	
	// 检查并创建索引：加速 sender 查询
	createIndexIfNotExistsMySQL("chat_messages", "idx_sender_created", "sender, created_at")
}

// createIndexIfNotExistsMySQL MySQL 兼容的索引创建函数
func createIndexIfNotExistsMySQL(tableName, indexName, columns string) {
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
			log.Printf("创建索引 %s 失败：%v", indexName, result.Error)
		} else {
			log.Printf("成功创建索引：%s", indexName)
		}
	} else {
		log.Printf("索引已存在：%s", indexName)
	}
}

// MigratePasswords 将所有明文密码升级为 bcrypt 哈希
func MigratePasswords() {
	var users []User
	DB.Find(&users)

	for _, user := range users {
		// 简单判断：如果密码长度小于 60，可能是明文
		if len(user.Password) < 60 {
			log.Printf("迁移用户 %s 的密码...", user.Username)
			hashed, err := HashPassword(user.Password)
			if err != nil {
				log.Printf("迁移失败：%v", err)
				continue
			}
			DB.Model(&user).Update("Password", hashed)
			log.Printf("✓ 用户 %s 密码迁移完成", user.Username)
		}
	}
	if len(users) > 0 {
		fmt.Println("密码迁移完成！")
	}
}
