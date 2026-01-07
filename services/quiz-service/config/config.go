package config

import (
	"os"
	"strconv"
)

type Config struct {
	Server   ServerConfig
	DB       DBConfig
	Redis    RedisConfig
	RabbitMQ RabbitMQConfig
	User	 UserServiceConfig
}

type ServerConfig struct {
	HTTPPort string
	GRPCPort string
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

type RabbitMQConfig struct {
	Host     string
	Port     string
	User     string
	Password string
}

type UserServiceConfig struct {
	Host string
	Port string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			HTTPPort: getEnv("HTTP_PORT", "8080"),
			GRPCPort: getEnv("GRPC_PORT", "50051"),
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
		RabbitMQ: RabbitMQConfig{
			Host:     getEnv("RABBITMQ_HOST", "rabbitmq"),
			Port:     getEnv("RABBITMQ_PORT", "5672"),
			User:     getEnv("RABBITMQ_USER", "admin"),
			Password: getEnv("RABBITMQ_PASSWORD", "admin"),
		},
		User: UserServiceConfig{
			Host: getEnv("USER_SERVICE_HOST", "localhost"),
			Port: getEnv("USER_SERVICE_PORT", "50051"),
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