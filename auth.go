package main

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"websocket-chat/config"
)

// 定义一个密钥，用于签名 JWT。从配置文件中读取。
var jwtKey []byte

// init 函数在包初始化时调用，用于加载 JWT 密钥
func init() {
	cfg, err := config.LoadConfig("config/config.json")
	if err != nil {
		// 如果配置文件加载失败，使用默认密钥
		jwtKey = []byte("my_secret_key")
		return
	}
	jwtKey = []byte(cfg.JWT.Secret)
}

// Claims 定义了 JWT 中存储的数据
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// GenerateJWT 为指定用户生成一个 JWT
func GenerateJWT(username string) (string, error) {
	// Token 的过期时间，例如 24 小时
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

// ValidateJWT 验证一个 JWT 并返回用户名
func ValidateJWT(tokenStr string) (string, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil {
		return "", err
	}

	if !token.Valid {
		return "", err
	}

	return claims.Username, nil
}
