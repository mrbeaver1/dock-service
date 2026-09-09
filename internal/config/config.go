package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPPort   uint16
	AdminToken string
	RPS        uint64
	Burst      uint64
	Postgres   PostgresConfig
	Redis      RedisConfig
	S3         S3Config
}

type PostgresConfig struct {
	Host     string
	Port     uint16
	User     string
	Password string
	Database string
	SSLMode  string
}

type RedisConfig struct {
	Host     string
	Port     uint16
	Password string
}

type S3Config struct {
	Host        string
	APIPort     uint16
	ConsolePort uint16
	AccessKey   string
	SecretKey   string
	Bucket      string
}

// Load reads a local .env file when it exists, then validates the effective
// environment. Existing environment variables take precedence over .env,
// which keeps Docker, CI, and production configuration authoritative.
func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	httpPort, err := requiredPort("APP_PORT")
	if err != nil {
		return Config{}, err
	}

	adminToken, err := requiredEnv("ADMIN_TOKEN")
	if err != nil {
		return Config{}, err
	}

	rps, err := requiredUint64("RPS")
	if err != nil {
		return Config{}, err
	}

	burst, err := requiredUint64("BURST")
	if err != nil {
		return Config{}, err
	}

	postgresHost, err := requiredEnv("POSTGRES_HOST")
	if err != nil {
		return Config{}, err
	}

	postgresPort, err := requiredPort("POSTGRES_PORT")
	if err != nil {
		return Config{}, err
	}

	postgresUser, err := requiredEnv("POSTGRES_USER")
	if err != nil {
		return Config{}, err
	}

	postgresPassword, err := requiredEnv("POSTGRES_PASSWORD")
	if err != nil {
		return Config{}, err
	}

	postgresDatabase, err := requiredEnv("POSTGRES_DB")
	if err != nil {
		return Config{}, err
	}

	redisHost, err := requiredEnv("REDIS_HOST")
	if err != nil {
		return Config{}, err
	}

	redisPort, err := requiredPort("REDIS_PORT")
	if err != nil {
		return Config{}, err
	}

	redisPassword, err := requiredEnv("REDIS_PASSWORD")
	if err != nil {
		return Config{}, err
	}

	s3Host, err := requiredEnv("S3_HOST")
	if err != nil {
		return Config{}, err
	}

	s3APIPort, err := requiredPort("S3_API_PORT")
	if err != nil {
		return Config{}, err
	}

	s3ConsolePort, err := requiredPort("S3_CONSOLE_PORT")
	if err != nil {
		return Config{}, err
	}

	s3AccessKey, err := requiredEnv("S3_ACCESS_KEY")
	if err != nil {
		return Config{}, err
	}

	s3SecretKey, err := requiredEnv("S3_SECRET_KEY")
	if err != nil {
		return Config{}, err
	}

	s3Bucket, err := requiredEnv("S3_BUCKET")
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPPort:   httpPort,
		AdminToken: adminToken,
		RPS:        rps,
		Burst:      burst,
		Postgres: PostgresConfig{
			Host:     postgresHost,
			Port:     postgresPort,
			User:     postgresUser,
			Password: postgresPassword,
			Database: postgresDatabase,
			SSLMode:  envOrDefault("POSTGRES_SSL_MODE", "prefer"),
		},
		Redis: RedisConfig{
			Host:     redisHost,
			Port:     redisPort,
			Password: redisPassword,
		},
		S3: S3Config{
			Host:        s3Host,
			APIPort:     s3APIPort,
			ConsolePort: s3ConsolePort,
			AccessKey:   s3AccessKey,
			SecretKey:   s3SecretKey,
			Bucket:      s3Bucket,
		},
	}, nil
}

func loadDotEnv() error {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load .env: %w", err)
	}

	return nil
}

func requiredEnv(key string) (string, error) {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return "", fmt.Errorf("missing required environment variable: %s", key)
	}

	return value, nil
}

func requiredPort(key string) (uint16, error) {
	value, err := requiredEnv(key)
	if err != nil {
		return 0, err
	}

	port, err := strconv.ParseUint(value, 10, 16)
	if err != nil || port == 0 {
		return 0, fmt.Errorf("%s must be a port from 1 to 65535", key)
	}

	return uint16(port), nil
}

func requiredUint64(key string) (uint64, error) {
	value, err := requiredEnv(key)
	if err != nil {
		return 0, err
	}

	number, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an unsigned integer", key)
	}

	return number, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
