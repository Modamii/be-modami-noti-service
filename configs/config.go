package configs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	logging "gitlab.com/lifegoeson-libs/pkg-logging"
)

// Config holds only what the notification service uses.
type Config struct {
	App        AppConfig        `mapstructure:"app"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Cache      CacheConfig      `mapstructure:"cache"`
	Kafka      KafkaConfig      `mapstructure:"kafka"`
	Queue      QueueConfig      `mapstructure:"queue"`
	Servers    ServersConfig    `mapstructure:"servers"`
	FCM        FCMConfig        `mapstructure:"fcm"`
	Centrifugo CentrifugoConfig `mapstructure:"centrifugo"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

type LoggingConfig struct {
	Level string `mapstructure:"level"`
}

type AppConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
	Debug       bool   `mapstructure:"debug"`
}

type DatabaseConfig struct {
	MongoDB MongoDBConfig `mapstructure:"mongodb"`
}

type MongoDBConfig struct {
	URI      string `mapstructure:"uri"`
	Database string `mapstructure:"database"`
}

type CacheConfig struct {
	Redis RedisCacheConfig `mapstructure:"redis"`
}

type RedisCacheConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type KafkaConfig struct {
	Brokers          []string `mapstructure:"brokers"`
	ConsumerGroupID  string   `mapstructure:"consumer_group_id"`
	NotificationTopic string `mapstructure:"notification_topic"`
}

type QueueConfig struct {
	WSKey   string `mapstructure:"ws_key"`
	PushKey string `mapstructure:"push_key"`
}

type ServersConfig struct {
	APIAddr    string `mapstructure:"api_addr"`
	IngestAddr string `mapstructure:"ingest_addr"`
}

type FCMConfig struct {
	CredentialsPath string `mapstructure:"credentials_path"`
}

type CentrifugoConfig struct {
	APIURL     string `mapstructure:"api_url"`
	APIKey     string `mapstructure:"api_key"`
	HMACSecret string `mapstructure:"hmac_secret"`
	TokenTTL   int    `mapstructure:"token_ttl"`
}

// Load reads config from Viper (config.yaml + optional config.{env}.yaml, env overrides).
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("/app/configs")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
		// No config file: use defaults from env/defaults below
	}

	env := viper.GetString("app.environment")
	if env == "" {
		env = "development"
	}
	envPath := filepath.Join("configs", fmt.Sprintf("config.%s.yaml", env))
	if _, err := os.Stat(envPath); err == nil {
		viper.SetConfigFile(envPath)
		if err := viper.MergeInConfig(); err != nil {
			return nil, fmt.Errorf("merge env config: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}


// ToLoggingConfig converts app config to the logging library config.
func (c *Config) ToLoggingConfig() logging.Config {
	return logging.Config{
		ServiceName:    c.App.Name,
		ServiceVersion: "1.0.0",
		Environment:    c.App.Environment,
		Level:          c.Logging.Level,
	}
}

func setDefaults() {
	viper.SetDefault("app.name", "notification-service")
	viper.SetDefault("app.environment", "development")
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("database.mongodb.uri", "mongodb://localhost:27017")
	viper.SetDefault("database.mongodb.database", "notifications")
	viper.SetDefault("cache.redis.addr", "localhost:6379")
	viper.SetDefault("queue.ws_key", "notif:ws")
	viper.SetDefault("queue.push_key", "notif:push")
	viper.SetDefault("servers.api_addr", ":8080")
	viper.SetDefault("servers.ingest_addr", ":8082")
	viper.SetDefault("centrifugo.api_url", "http://localhost:8000/api")
	viper.SetDefault("centrifugo.token_ttl", 3600)
}

func validateConfig(cfg *Config) error {
	if cfg.Queue.WSKey == "" || cfg.Queue.PushKey == "" {
		return fmt.Errorf("queue ws_key and push_key are required")
	}
	return nil
}

// RedisURL returns connection string for Redis (used by go-redis).
func (c *Config) RedisURL() string {
	addr := c.Cache.Redis.Addr
	if addr == "" {
		addr = "localhost:6379"
	}
	if c.Cache.Redis.Password != "" {
		return "redis://:" + c.Cache.Redis.Password + "@" + addr
	}
	if strings.HasPrefix(addr, "redis://") || strings.HasPrefix(addr, "rediss://") {
		return addr
	}
	return "redis://" + addr
}

// RedisOptions returns redis.Options for the configured Redis.
func RedisOptions(cfg *Config) *redis.Options {
	url := cfg.RedisURL()
	if strings.HasPrefix(url, "redis://") || strings.HasPrefix(url, "rediss://") {
		opts, err := redis.ParseURL(url)
		if err != nil {
			return &redis.Options{Addr: "localhost:6379"}
		}
		return opts
	}
	return &redis.Options{Addr: cfg.Cache.Redis.Addr, Password: cfg.Cache.Redis.Password, DB: cfg.Cache.Redis.DB}
}
