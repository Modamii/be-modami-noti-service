package configs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	logging "gitlab.com/lifegoeson-libs/pkg-logging"
)

// Config holds all configuration for the notification service.
type Config struct {
	App           AppConfig           `mapstructure:"app"`
	MongoDB       MongoDBConfig       `mapstructure:"mongodb"`
	Redis         RedisConfig         `mapstructure:"redis"`
	Kafka         KafkaConfig         `mapstructure:"kafka"`
	Queue         QueueConfig         `mapstructure:"queue"`
	Servers       ServersConfig       `mapstructure:"servers"`
	FCM           FCMConfig           `mapstructure:"fcm"`
	Centrifugo    CentrifugoConfig    `mapstructure:"centrifugo"`
	Observability ObservabilityConfig `mapstructure:"observability"`
}

type AppConfig struct {
	Name            string   `mapstructure:"name"`
	Version         string   `mapstructure:"version"`
	Environment     string   `mapstructure:"environment"`
	Debug           bool     `mapstructure:"debug"`
	Port            int      `mapstructure:"port"`
	Host            string   `mapstructure:"host"`
	SwaggerHost     string   `mapstructure:"swagger_host"`
	ShutdownTimeout string   `mapstructure:"shutdown_timeout"`
	ReadTimeout     string   `mapstructure:"read_timeout"`
	WriteTimeout    string   `mapstructure:"write_timeout"`
	IdleTimeout     string   `mapstructure:"idle_timeout"`
	CORSOrigins     []string `mapstructure:"cors_origins"`
}

func (a *AppConfig) ParseShutdownTimeout() time.Duration {
	return parseDuration(a.ShutdownTimeout, 30*time.Second)
}

func (a *AppConfig) ParseReadTimeout() time.Duration {
	return parseDuration(a.ReadTimeout, 30*time.Second)
}

func (a *AppConfig) ParseWriteTimeout() time.Duration {
	return parseDuration(a.WriteTimeout, 30*time.Second)
}

func (a *AppConfig) ParseIdleTimeout() time.Duration {
	return parseDuration(a.IdleTimeout, 120*time.Second)
}

type MongoDBConfig struct {
	URI           string `mapstructure:"uri"`
	Database      string `mapstructure:"database"`
	Timeout       string `mapstructure:"timeout"`
	MaxPoolSize   uint64 `mapstructure:"max_pool_size"`
	MinPoolSize   uint64 `mapstructure:"min_pool_size"`
	MaxIdleTime   string `mapstructure:"max_idle_time"`
	RetryWrites   bool   `mapstructure:"retry_writes"`
	ReadConcern   string `mapstructure:"read_concern"`
	WriteConcern  string `mapstructure:"write_concern"`
	EnableLogging bool   `mapstructure:"enable_logging"`
}

type RedisConfig struct {
	Host              string         `mapstructure:"host"`
	Port              int            `mapstructure:"port"`
	Database          int            `mapstructure:"database"`
	RateLimitDatabase int            `mapstructure:"rate_limit_database"`
	TTL               string         `mapstructure:"ttl"`
	PoolSize          int            `mapstructure:"pool_size"`
	Pass              string         `mapstructure:"pass"`
	UserName          string         `mapstructure:"user_name"`
	WriteTimeout      string         `mapstructure:"write_timeout"`
	ReadTimeout       string         `mapstructure:"read_timeout"`
	DialTimeout       string         `mapstructure:"dial_timeout"`
	TLSConfig         RedisTLSConfig `mapstructure:"tls_config"`
}

type RedisTLSConfig struct {
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`
}

type KafkaConfig struct {
	BrokerList             string `mapstructure:"broker_list"`
	Enable                 bool   `mapstructure:"enable"`
	TLSEnable              bool   `mapstructure:"tls_enable"`
	Partition              int    `mapstructure:"partition"`
	Partitioner            string `mapstructure:"partitioner"`
	SASLProducerUsername   string `mapstructure:"sasl_producer_username"`
	SASLProducerPassword   string `mapstructure:"sasl_producer_password"`
	SASLConsumerUsername   string `mapstructure:"sasl_consumer_username"`
	SASLConsumerPassword   string `mapstructure:"sasl_consumer_password"`
	UserActivatedTopicName string `mapstructure:"user_activated_topic_name"`
	ConsumerGroup          string `mapstructure:"consumer_group"`
	ClientID               string `mapstructure:"client_id"`
	Env                    string `mapstructure:"env"`
}

// Brokers splits BrokerList by comma and returns a slice.
func (k *KafkaConfig) Brokers() []string {
	if k.BrokerList == "" {
		return nil
	}
	var brokers []string
	for _, b := range strings.Split(k.BrokerList, ",") {
		if b = strings.TrimSpace(b); b != "" {
			brokers = append(brokers, b)
		}
	}
	return brokers
}

type QueueConfig struct {
	WSKey   string `mapstructure:"ws_key"`
	PushKey string `mapstructure:"push_key"`
}

type ServersConfig struct {
	APIAddr     string `mapstructure:"api_addr"`
	IngestAddr  string `mapstructure:"ingest_addr"`
	GatewayAddr string `mapstructure:"gateway_addr"`
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

type ObservabilityConfig struct {
	ServiceName    string `mapstructure:"service_name"`
	ServiceVersion string `mapstructure:"service_version"`
	Environment    string `mapstructure:"environment"`
	LogLevel       string `mapstructure:"log_level"`
	OTLPEndpoint   string `mapstructure:"otlp_endpoint"`
	OTLPInsecure   bool   `mapstructure:"otlp_insecure"`
}

// Load reads config from ./config/config.yaml (+ optional config.{env}.yaml, env overrides).
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/app/config")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	env := viper.GetString("app.environment")
	if env == "" {
		env = "development"
	}
	envPath := filepath.Join("config", fmt.Sprintf("config.%s.yaml", env))
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

// ToLoggingConfig converts observability config to the logging library config.
func (c *Config) ToLoggingConfig() logging.Config {
	return logging.Config{
		ServiceName:    c.Observability.ServiceName,
		ServiceVersion: c.Observability.ServiceVersion,
		Environment:    c.Observability.Environment,
		Level:          c.Observability.LogLevel,
	}
}

// RedisAddr returns host:port for the Redis connection.
func (c *Config) RedisAddr() string {
	host := c.Redis.Host
	if host == "" {
		host = "localhost"
	}
	port := c.Redis.Port
	if port == 0 {
		port = 6379
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// RedisOptions returns redis.Options for the configured Redis.
func RedisOptions(cfg *Config) *redis.Options {
	opts := &redis.Options{
		Addr:     cfg.RedisAddr(),
		Password: cfg.Redis.Pass,
		DB:       cfg.Redis.Database,
		PoolSize: cfg.Redis.PoolSize,
		Username: cfg.Redis.UserName,
	}
	if cfg.Redis.WriteTimeout != "" {
		opts.WriteTimeout = parseDuration(cfg.Redis.WriteTimeout, 600*time.Second)
	}
	if cfg.Redis.ReadTimeout != "" {
		opts.ReadTimeout = parseDuration(cfg.Redis.ReadTimeout, 600*time.Second)
	}
	if cfg.Redis.DialTimeout != "" {
		opts.DialTimeout = parseDuration(cfg.Redis.DialTimeout, 600*time.Second)
	}
	return opts
}

func setDefaults() {
	viper.SetDefault("app.name", "modami-notification-service")
	viper.SetDefault("app.version", "1.0.0")
	viper.SetDefault("app.environment", "development")
	viper.SetDefault("app.port", 7070)
	viper.SetDefault("app.swagger_host", "localhost:7070")
	viper.SetDefault("app.shutdown_timeout", "30s")
	viper.SetDefault("app.read_timeout", "30s")
	viper.SetDefault("app.write_timeout", "30s")
	viper.SetDefault("app.idle_timeout", "120s")
	viper.SetDefault("app.cors_origins", []string{"*"})

	viper.SetDefault("mongodb.uri", "mongodb://localhost:27017")
	viper.SetDefault("mongodb.database", "notifications")
	viper.SetDefault("mongodb.timeout", "10s")
	viper.SetDefault("mongodb.max_pool_size", 100)
	viper.SetDefault("mongodb.retry_writes", true)

	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.pool_size", 10)

	viper.SetDefault("queue.ws_key", "notif:ws")
	viper.SetDefault("queue.push_key", "notif:push")

	viper.SetDefault("servers.api_addr", ":7070")
	viper.SetDefault("servers.ingest_addr", ":7071")
	viper.SetDefault("servers.gateway_addr", ":7072")

	viper.SetDefault("centrifugo.api_url", "http://localhost:8000/api")
	viper.SetDefault("centrifugo.token_ttl", 3600)

	viper.SetDefault("observability.service_name", "modami-notification-service")
	viper.SetDefault("observability.service_version", "1.0.0")
	viper.SetDefault("observability.log_level", "info")
	viper.SetDefault("observability.otlp_insecure", true)
}

func validateConfig(cfg *Config) error {
	if cfg.Queue.WSKey == "" || cfg.Queue.PushKey == "" {
		return fmt.Errorf("queue ws_key and push_key are required")
	}
	return nil
}

func parseDuration(s string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return fallback
	}
	return d
}
