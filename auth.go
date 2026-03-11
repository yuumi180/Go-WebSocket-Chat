package main

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// 定义一个密钥，用于签名 JWT。在生产环境中应该更复杂，并从配置中读取。
var jwtKey = []byte("my_secret_key")

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
