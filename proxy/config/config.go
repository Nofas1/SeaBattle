package config

import (
	"os"

	"github.com/ilyakaznacheev/cleanenv"
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

	AdminUser string `env:"ADMIN_USER"`
	AdminPass string `env:"ADMIN_PASSWORD"`
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
	if err = cleanenv.ReadEnv(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}