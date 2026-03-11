package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	JWT      JWTConfig      `json:"jwt"`
}

type ServerConfig struct {
	Port string `json:"port"`
	Host string `json:"host"`
}

type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
}

type JWTConfig struct {
	Secret     string `json:"secret"`
	Expiration int    `json:"expiration"` // 小时
}

var GlobalConfig *Config

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := &Config{}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(config)
	if err != nil {
		return nil, err
	}

	GlobalConfig = config
	return config, nil
}
