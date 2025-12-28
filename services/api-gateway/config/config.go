package config

import (
	"os"
)

type Config struct {
	Server ServerConfig
	Auth   AuthServiceConfig
	JWT    JWTConfig
}

type ServerConfig struct {
	Port string
	Host string
}

type AuthServiceConfig struct {
	Host string
	Port string
}

type JWTConfig struct {
	Secret string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
		},
		Auth: AuthServiceConfig{
			Host: getEnv("AUTH_SERVICE_HOST", "localhost"),
			Port: getEnv("AUTH_SERVICE_PORT", "50051"),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", "test-secret-key"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (c *Config) GetServerAddress() string {
	return c.Server.Host + ":" + c.Server.Port
}