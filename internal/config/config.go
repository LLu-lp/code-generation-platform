// Package config — 配置管理（Viper）
//
// 使用 Viper 加载 YAML 配置文件，支持环境变量覆盖。
// Config 结构体包含所有子配置：Server、Database、Redis、Session、AI（4 模型）、COS、Pexels、Code、RateLimit。
package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

// Config 全局配置
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Session   SessionConfig   `mapstructure:"session"`
	AI        AIConfig        `mapstructure:"ai"`
	COS       COSConfig       `mapstructure:"cos"`
	Pexels    PexelsConfig    `mapstructure:"pexels"`
	Code      CodeConfig      `mapstructure:"code"`
	RateLimit RateLimitConfig `mapstructure:"rate-limit"`
	Swagger   SwaggerConfig   `mapstructure:"swagger"`
}

type ServerConfig struct {
	Port        int    `mapstructure:"port"`
	ContextPath string `mapstructure:"context-path"`
	Mode        string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Driver               string `mapstructure:"driver"`
	Host                 string `mapstructure:"host"`
	Port                 int    `mapstructure:"port"`
	Database             string `mapstructure:"database"`
	Username             string `mapstructure:"username"`
	Password             string `mapstructure:"password"`
	Charset              string `mapstructure:"charset"`
	MaxIdleConns         int    `mapstructure:"max-idle-conns"`
	MaxOpenConns         int    `mapstructure:"max-open-conns"`
	ConnMaxLifetimeMinutes int  `mapstructure:"conn-max-lifetime-minutes"`
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		d.Username, d.Password, d.Host, d.Port, d.Database, d.Charset)
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	TTL      int64  `mapstructure:"ttl"`
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type SessionConfig struct {
	StoreType      string `mapstructure:"store-type"`
	TimeoutSeconds int    `mapstructure:"timeout-seconds"`
	CookieMaxAge   int    `mapstructure:"cookie-max-age"`
}

// AIModelConfig 单个 AI 模型配置
type AIModelConfig struct {
	Provider     string  `mapstructure:"provider"`
	BaseURL      string  `mapstructure:"base-url"`
	APIKey       string  `mapstructure:"api-key"`
	ModelName    string  `mapstructure:"model-name"`
	MaxTokens    int     `mapstructure:"max-tokens"`
	Temperature  float64 `mapstructure:"temperature"`
	LogRequests  bool    `mapstructure:"log-requests"`
	LogResponses bool    `mapstructure:"log-responses"`
	MaxRetries   int     `mapstructure:"max-retries"`
}

type AIConfig struct {
	ChatModel      AIModelConfig `mapstructure:"chat-model"`
	StreamingModel AIModelConfig `mapstructure:"streaming-chat-model"`
	ReasoningModel AIModelConfig `mapstructure:"reasoning-model"`
	RoutingModel   AIModelConfig `mapstructure:"routing-model"`
	ImageModel     struct {
		APIKey    string `mapstructure:"api-key"`
		ModelName string `mapstructure:"model-name"`
	} `mapstructure:"image-model"`
}

type COSConfig struct {
	Host      string `mapstructure:"host"`
	SecretID  string `mapstructure:"secret-id"`
	SecretKey string `mapstructure:"secret-key"`
	Region    string `mapstructure:"region"`
	Bucket    string `mapstructure:"bucket"`
}

type PexelsConfig struct {
	APIKey string `mapstructure:"api-key"`
}

type CodeConfig struct {
	OutputRootDir string `mapstructure:"output-root-dir"`
	DeployRootDir string `mapstructure:"deploy-root-dir"`
	DeployHost    string `mapstructure:"deploy-host"`
}

type RateLimitConfig struct {
	AIChatRate            int `mapstructure:"ai-chat-rate"`
	AIChatIntervalSeconds int `mapstructure:"ai-chat-interval-seconds"`
	DefaultRate           int `mapstructure:"default-rate"`
	DefaultIntervalSeconds int `mapstructure:"default-interval-seconds"`
}

type SwaggerConfig struct {
	Enable   bool   `mapstructure:"enable"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

var AppConfig *Config

// LoadConfig 加载配置
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// 环境变量覆盖
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	AppConfig = &cfg
	log.Println("[Config] 配置加载成功")
	return &cfg, nil
}
