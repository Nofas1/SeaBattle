package config

import (
	"os"

	"gopkg.in/yaml.v3"
)


type Config struct {
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
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
