package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type BotConfig struct {
	URL string `yaml:"url"`
}

type ProxyConfig struct {
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
}

type Config struct {
	Proxy ProxyConfig          `yaml:"proxy"`
	Bots  map[string]BotConfig `yaml:"bots"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err = yaml.Unmarshal(file, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
