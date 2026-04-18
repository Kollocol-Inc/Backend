package config

import (
	"os"
	"strings"
)

type Config struct {
	Server       ServerConfig
	Auth         AuthServiceConfig
	User         UserServiceConfig
	Quiz         QuizServiceConfig
	Game         GameServiceConfig
	Notification NotificationServiceConfig
	ML           MLServiceConfig
	JWT          JWTConfig
}

type ServerConfig struct {
	Port string
	Host string
}

type AuthServiceConfig struct {
	Host string
	Port string
}

type UserServiceConfig struct {
	Host string
	Port string
}

type QuizServiceConfig struct {
	Host string
	Port string
}

type GameServiceConfig struct {
	Host string
	Port string
}

type NotificationServiceConfig struct {
	Host string
	Port string
}

type MLServiceConfig struct {
	Host    string
	Port    string
	Enabled bool
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
		User: UserServiceConfig{
			Host: getEnv("USER_SERVICE_HOST", "localhost"),
			Port: getEnv("USER_SERVICE_PORT", "50051"),
		},
		Quiz: QuizServiceConfig{
			Host: getEnv("QUIZ_SERVICE_HOST", "localhost"),
			Port: getEnv("QUIZ_SERVICE_PORT", "50051"),
		},
		Game: GameServiceConfig{
			Host: getEnv("GAME_SERVICE_HOST", "localhost"),
			Port: getEnv("GAME_SERVICE_PORT", "8081"),
		},
		Notification: NotificationServiceConfig{
			Host: getEnv("NOTIFICATION_SERVICE_HOST", "localhost"),
			Port: getEnv("NOTIFICATION_SERVICE_PORT", "50051"),
		},
		ML: MLServiceConfig{
			Host:    getEnv("ML_SERVICE_HOST", "localhost"),
			Port:    getEnv("ML_SERVICE_PORT", "50051"),
			Enabled: getEnvBool("AI_FEATURES_ENABLED", true),
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

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return strings.EqualFold(value, "true") || value == "1"
	}
	return defaultValue
}

func (c *Config) GetServerAddress() string {
	return c.Server.Host + ":" + c.Server.Port
}
