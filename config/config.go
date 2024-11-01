package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	App      AppConfig
}

type ServerConfig struct {
	Host         string
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type DatabaseConfig struct {
	Host               string
	Port               string
	User               string
	Password           string
	DBName             string
	SSLMode            string
	MaxConnections     int
	MaxIdleConnections int
}

type AppConfig struct {
	TransactionHistoryLimit int
	MinTransactionAmount    float64
	MaxTransactionAmount    float64
}

func Load() (*Config, error) {
	readTimeout, _ := time.ParseDuration(getEnv("WALLET_SERVER_READ_TIMEOUT", "15s"))
	writeTimeout, _ := time.ParseDuration(getEnv("WALLET_SERVER_WRITE_TIMEOUT", "15s"))
	idleTimeout, _ := time.ParseDuration(getEnv("WALLET_SERVER_IDLE_TIMEOUT", "60s"))

	maxConn, _ := strconv.Atoi(getEnv("WALLET_DATABASE_MAX_CONNECTIONS", "100"))
	maxIdleConn, _ := strconv.Atoi(getEnv("WALLET_DATABASE_MAX_IDLE_CONNECTIONS", "10"))

	historyLimit, _ := strconv.Atoi(getEnv("WALLET_TRANSACTION_HISTORY_LIMIT", "100"))
	minAmount, _ := strconv.ParseFloat(getEnv("WALLET_MIN_TRANSACTION_AMOUNT", "0.01"), 64)
	maxAmount, _ := strconv.ParseFloat(getEnv("WALLET_MAX_TRANSACTION_AMOUNT", "1000000"), 64)

	return &Config{
		Server: ServerConfig{
			Host:         getEnv("WALLET_SERVER_HOST", "0.0.0.0"),
			Port:         getEnv("WALLET_SERVER_PORT", "8080"),
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
			IdleTimeout:  idleTimeout,
		},
		Database: DatabaseConfig{
			Host:               getEnv("WALLET_DATABASE_HOST", "db"),
			Port:               getEnv("WALLET_DATABASE_PORT", "5432"),
			User:               getEnv("WALLET_DATABASE_USER", "postgres"),
			Password:           getEnv("WALLET_DATABASE_PASSWORD", "password"),
			DBName:             getEnv("WALLET_DATABASE_NAME", "wallet"),
			SSLMode:            getEnv("WALLET_DATABASE_SSLMODE", "disable"),
			MaxConnections:     maxConn,
			MaxIdleConnections: maxIdleConn,
		},
		App: AppConfig{
			TransactionHistoryLimit: historyLimit,
			MinTransactionAmount:    minAmount,
			MaxTransactionAmount:    maxAmount,
		},
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}
