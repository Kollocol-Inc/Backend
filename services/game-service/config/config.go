package config

import (
	"os"
	"strconv"
)

type Config struct {
	Server   ServerConfig
	DB       DBConfig
	Redis    RedisConfig
	Quiz     QuizServiceConfig
	Auth     AuthServiceConfig
}

type ServerConfig struct {
	HTTPPort string
	WSPort   string
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type QuizServiceConfig struct {
	Host string
	Port string
}

type AuthServiceConfig struct {
	Host string
	Port string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			HTTPPort: getEnv("HTTP_PORT", "8080"),
			WSPort:   getEnv("WS_PORT", "8081"),
		},
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "postgres"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "kollocol"),
			Password: getEnv("DB_PASSWORD", "kollocol_password"),
			DBName:   getEnv("DB_NAME", "kollocol"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "redis"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		Quiz: QuizServiceConfig{
			Host: getEnv("QUIZ_SERVICE_HOST", "localhost"),
			Port: getEnv("QUIZ_SERVICE_PORT", "50051"),
		},
		Auth: AuthServiceConfig{
			Host: getEnv("AUTH_SERVICE_HOST", "localhost"),
			Port: getEnv("AUTH_SERVICE_PORT", "50051"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}