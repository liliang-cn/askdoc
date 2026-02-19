package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config holds all configuration for AskDoc
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Admin    AdminConfig    `mapstructure:"admin"`
	Database DatabaseConfig `mapstructure:"database"`
	Storage  StorageConfig  `mapstructure:"storage"`
	RAG      RAGConfig      `mapstructure:"rag"`
	LLM      LLMConfig      `mapstructure:"llm"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
	BaseURL string `mapstructure:"base_url"`
}

// AdminConfig holds admin authentication configuration
type AdminConfig struct {
	APIKey string `mapstructure:"api_key"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

// StorageConfig holds document storage configuration
type StorageConfig struct {
	Documents string `mapstructure:"documents"`
}

// RAGConfig holds RAG configuration
type RAGConfig struct {
	DBPath       string `mapstructure:"db_path"`
	IndexType    string `mapstructure:"index_type"`
	ChunkSize    int    `mapstructure:"chunk_size"`
	ChunkOverlap int    `mapstructure:"chunk_overlap"`
}

// LLMConfig holds LLM provider configuration
type LLMConfig struct {
	Provider       string `mapstructure:"provider"`
	BaseURL        string `mapstructure:"base_url"`
	APIKey         string `mapstructure:"api_key"`
	EmbeddingModel string `mapstructure:"embedding_model"`
	LLMModel       string `mapstructure:"llm_model"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled         bool `mapstructure:"enabled"`
	RequestsPerHour int  `mapstructure:"requests_per_hour"`
}

// Load loads configuration from file and environment
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Read config file if specified
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
	}

	// Environment variables
	v.SetEnvPrefix("ASKDOC")
	v.AutomaticEnv()

	// Read config
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
		// Config file not found, use defaults
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.base_url", "http://localhost:8080")

	v.SetDefault("admin.api_key", "")

	v.SetDefault("database.path", "./data/askdoc.db")
	v.SetDefault("storage.documents", "./data/documents")

	v.SetDefault("rag.db_path", "./data/rag.db")
	v.SetDefault("rag.index_type", "hnsw")
	v.SetDefault("rag.chunk_size", 1000)
	v.SetDefault("rag.chunk_overlap", 200)

	v.SetDefault("llm.provider", "ollama")
	v.SetDefault("llm.base_url", "http://localhost:11434/v1")
	v.SetDefault("llm.api_key", "")
	v.SetDefault("llm.embedding_model", "nomic-embed-text")
	v.SetDefault("llm.llm_model", "qwen2.5:7b")

	v.SetDefault("rate_limit.enabled", true)
	v.SetDefault("rate_limit.requests_per_hour", 100)
}

// Address returns the server address
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
