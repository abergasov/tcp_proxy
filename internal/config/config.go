package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"tcp_proxy/internal/service/proxier"

	"gopkg.in/yaml.v2"
)

type AppConfig struct {
	BoxName      string           `yaml:"box_name"`
	SlackHookURL string           `yaml:"slack_hook_url"`
	ProxyList    []proxier.Config `yaml:"proxy_list"`
}

func LoadConfig(confFile string) (*AppConfig, error) {
	file, err := os.Open(filepath.Clean(confFile))
	if err != nil {
		return nil, fmt.Errorf("error open config file: %w", err)
	}
	defer func() {
		if e := file.Close(); e != nil {
			log.Fatal("Error close config file", e)
		}
	}()

	var cfg AppConfig
	if err = yaml.NewDecoder(file).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("error decode config file: %w", err)
	}

	return &cfg, nil
}
