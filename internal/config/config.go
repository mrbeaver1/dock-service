package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPPort            uint16
	WriteIdleTimeout    time.Duration
	AdminToken          string
	JWTSecret           string
	CacheTTL            time.Duration
	RPS                 uint64
	Burst               uint64
	PasswordConcurrency int
	Upload              UploadConfig
	Credentials         BodyConfig
	Postgres            PostgresConfig
	Redis               RedisConfig
	S3                  S3Config
}

type BodyConfig struct {
	Concurrent  int
	MaxBytes    int64
	IdleTimeout time.Duration
	ReadTimeout time.Duration
}

type UploadConfig struct {
	BodyConfig
	Timeout time.Duration
	TempDir string
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
	jwtSecret, err := requiredEnv("JWT_SECRET")
	if err != nil {
		return Config{}, err
	}
	cacheTTL, err := time.ParseDuration(envOrDefault("CACHE_TTL", "5m"))
	if err != nil || cacheTTL < time.Millisecond {
		return Config{}, fmt.Errorf("CACHE_TTL must be a duration of at least one millisecond")
	}
	writeIdleTimeout, err := positiveDuration("HTTP_WRITE_IDLE_TIMEOUT", "30s")
	if err != nil {
		return Config{}, err
	}
	passwordConcurrency, err := positiveInt("PASSWORD_MAX_CONCURRENT", 2)
	if err != nil {
		return Config{}, err
	}
	upload, err := uploadFromEnv()
	if err != nil {
		return Config{}, err
	}

	credentials, err := bodyFromEnv("AUTH", 8, "8388608", "10s", "30s")
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

	postgres, err := postgresFromEnv()
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
		HTTPPort:            httpPort,
		WriteIdleTimeout:    writeIdleTimeout,
		AdminToken:          adminToken,
		JWTSecret:           jwtSecret,
		CacheTTL:            cacheTTL,
		RPS:                 rps,
		Burst:               burst,
		PasswordConcurrency: passwordConcurrency,
		Upload:              upload,
		Credentials:         credentials,
		Postgres:            postgres,
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

func uploadFromEnv() (UploadConfig, error) {
	var cfg UploadConfig
	var err error
	if cfg.BodyConfig, err = bodyFromEnv("UPLOAD", 4, "1073741824", "30s", "30m"); err != nil {
		return cfg, err
	}
	if cfg.Timeout, err = positiveDuration("UPLOAD_TIMEOUT", "30m"); err != nil {
		return cfg, err
	}
	cfg.TempDir = envOrDefault("UPLOAD_TEMP_DIR", filepath.Join(os.TempDir(), "dock-service-uploads"))
	return cfg, nil
}

func bodyFromEnv(prefix string, concurrent int, bytes, idle, timeout string) (BodyConfig, error) {
	var cfg BodyConfig
	var err error
	if cfg.Concurrent, err = positiveInt(prefix+"_MAX_CONCURRENT", concurrent); err != nil {
		return cfg, err
	}
	if cfg.MaxBytes, err = strconv.ParseInt(envOrDefault(prefix+"_MAX_BYTES", bytes), 10, 64); err != nil || cfg.MaxBytes < 1 {
		return cfg, fmt.Errorf("%s_MAX_BYTES must be a positive number of bytes", prefix)
	}
	if cfg.IdleTimeout, err = positiveDuration(prefix+"_IDLE_TIMEOUT", idle); err != nil {
		return cfg, err
	}
	if cfg.ReadTimeout, err = positiveDuration(prefix+"_READ_TIMEOUT", timeout); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func positiveInt(key string, fallback int) (int, error) {
	value, err := strconv.Atoi(envOrDefault(key, strconv.Itoa(fallback)))
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return value, nil
}

func positiveDuration(key, fallback string) (time.Duration, error) {
	value, err := time.ParseDuration(envOrDefault(key, fallback))
	if err != nil || value < time.Millisecond {
		return 0, fmt.Errorf("%s must be a duration of at least one millisecond", key)
	}
	return value, nil
}

func LoadPostgres() (PostgresConfig, error) {
	if err := loadDotEnv(); err != nil {
		return PostgresConfig{}, err
	}
	return postgresFromEnv()
}

func postgresFromEnv() (PostgresConfig, error) {
	var cfg PostgresConfig
	var err error
	if cfg.Host, err = requiredEnv("POSTGRES_HOST"); err != nil {
		return cfg, err
	}
	if cfg.Port, err = requiredPort("POSTGRES_PORT"); err != nil {
		return cfg, err
	}
	if cfg.User, err = requiredEnv("POSTGRES_USER"); err != nil {
		return cfg, err
	}
	if cfg.Password, err = requiredEnv("POSTGRES_PASSWORD"); err != nil {
		return cfg, err
	}
	if cfg.Database, err = requiredEnv("POSTGRES_DB"); err != nil {
		return cfg, err
	}
	cfg.SSLMode = envOrDefault("POSTGRES_SSL_MODE", "prefer")
	return cfg, nil
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
