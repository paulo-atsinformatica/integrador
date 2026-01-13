package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	NodeID   string `yaml:"node_id"`
	Firebird struct {
		DSN     string `yaml:"dsn"`
		AppName string `yaml:"app_name"`
	} `yaml:"firebird"`
	Trace struct {
		Enabled       bool   `yaml:"enabled"`
		FBTraceMgrPos string `yaml:"fbtracemgr_path"`
		LogPath       string `yaml:"log_path"` // Caminho do arquivo de log do System Audit
		PollInterval  int    `yaml:"poll_interval"`
	} `yaml:"trace"`
	Webhook struct {
		ListenAddr string `yaml:"listen_addr"`
		RemoteURL  string `yaml:"remote_url"`
		Token      string `yaml:"token"`
	} `yaml:"webhook"`
	Relay struct {
		Enabled bool   `yaml:"enabled"`
		HubURL  string `yaml:"hub_url"`
		Token   string `yaml:"token"`
	} `yaml:"relay"`
	Integracao struct {
		BatchSize            int `yaml:"batch_size"`
		RetryMax             int `yaml:"retry_max"`
		RetryIntervalSeconds int `yaml:"retry_interval_seconds"`
		TimeoutSeconds       int `yaml:"timeout_seconds"`
	} `yaml:"integracao"`
}

// Load lê o arquivo de configuração e retorna um objeto Config
func Load(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir arquivo de config: %w", err)
	}
	defer file.Close()

	var cfg Config
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("erro ao decodificar config YAML: %w", err)
	}

	// Validações básicas
	if cfg.Firebird.DSN == "" {
		return nil, fmt.Errorf("firebird.dsn é obrigatório")
	}

	return &cfg, nil
}

// Save grava a configuração no caminho especificado
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("erro ao converter config para YAML: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("erro ao gravar arquivo de config: %w", err)
	}

	return nil
}
