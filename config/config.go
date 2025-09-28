package config

import (
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Redis      RedisConfig      `envconfig:"REDIS"`
	Blockchain BlockchainConfig `envconfig:"BLOCKCHAIN"`
	Service    ServiceConfig    `envconfig:"SERVICE"`
	Log        LogConfig        `envconfig:"LOG"`
}

type RedisConfig struct {
	Host     string `envconfig:"HOST"     default:"localhost"`
	Port     int    `envconfig:"PORT"     default:"6379"`
	Password string `envconfig:"PASSWORD" default:""`
	DB       int    `envconfig:"DB"       default:"0"`
}

type BlockchainConfig struct {
	RPCURL    string `envconfig:"RPC_URL"    default:"https://rpc.plasma.network"`
	WSURL     string `envconfig:"WS_URL"     default:"wss://ws.plasma.network"`
	ChainID   int64  `envconfig:"CHAIN_ID"   default:"9745"`
	BatchSize int    `envconfig:"BATCH_SIZE" default:"100"`
}

type ServiceConfig struct {
	CommandChannel      string `envconfig:"COMMAND_CHANNEL"      default:"wallet_commands"`
	NotificationChannel string `envconfig:"NOTIFICATION_CHANNEL" default:"wallet_notifications"`
	WorkerCount         int    `envconfig:"WORKER_COUNT"         default:"10"`
}

type LogConfig struct {
	Level  string `envconfig:"LOG_LEVEL"  default:"info"`
	Format string `envconfig:"LOG_FORMAT" default:"json"`
}

func Load() (*Config, error) {
	var cfg Config
	err := envconfig.Process("", &cfg)
	return &cfg, err
}
