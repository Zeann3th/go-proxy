package main

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type ProxyConfig struct {
	Name    string `yaml:"name"`
	URL     string `yaml:"url"`
	Timeout int    `yaml:"timeout"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Proxies []ProxyConfig `yaml:"proxies"`
}

func (c *Config) Validate() error {
	if c.Server.Host == "" || c.Server.Port == 0 {
		return fmt.Errorf("[Config] server.host and server.port is required")
	}
	if len(c.Proxies) == 0 {
		return fmt.Errorf("[Config] at least one proxy is required")
	}

	for _, p := range c.Proxies {
		if p.URL == "" {
			return fmt.Errorf("[Config] proxies must have a valid url")
		}
	}

	return nil
}

func NewConfig(filePath string) *Config {
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatal(err)
	}

	var config Config

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatal(err)
	}

	err = config.Validate()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Go Proxy server started at %s:%d\n", config.Server.Host, config.Server.Port)

	for _, p := range config.Proxies {
		fmt.Printf("Proxy [%s] -> %s (Timeout: %ds)\n", p.Name, p.URL, p.Timeout)
	}

	return &config
}
