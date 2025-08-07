package main

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"os"
)

type Config struct {
	LocalDirectory  string `yaml:"local_directory"`
	RemoteDirectory string `yaml:"remote_directory"`
	SSH             struct {
		Host           string `yaml:"host"`
		Port           int    `yaml:"port"`
		Username       string `yaml:"username"`
		Password       string `yaml:"password"`
		PrivateKeyPath string `yaml:"private_key_path"`
	} `yaml:"ssh"`
	Sync struct {
		IgnorePatterns []string `yaml:"ignore_patterns"`
		DeleteRemote   bool     `yaml:"delete_remote"`
		SyncInterval   int      `yaml:"sync_interval"`
	} `yaml:"sync"`
	Log struct {
		Level     string `yaml:"level"`
		File      string `yaml:"file"`
		MaxSizeMB int    `yaml:"max_size_mb"`
	} `yaml:"log"`
}

func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("配置文件打开失败: %v", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("配置文件读取失败: %v", err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("配置文件解析失败: %v", err)
	}

	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %v", err)
	}

	return &config, nil
}

func validateConfig(config *Config) error {
	if config.LocalDirectory == "" {
		return fmt.Errorf("本地目录不能为空")
	}
	if config.RemoteDirectory == "" {
		return fmt.Errorf("远程目录不能为空")
	}
	if config.SSH.Host == "" {
		return fmt.Errorf("SSH主机不能为空")
	}
	if config.SSH.Username == "" {
		return fmt.Errorf("SSH用户名不能为空")
	}
	if config.SSH.Password == "" && config.SSH.PrivateKeyPath == "" {
		return fmt.Errorf("必须提供SSH密码或私钥路径")
	}
	if config.SSH.Port <= 0 {
		config.SSH.Port = 22
	}
	if config.Sync.SyncInterval <= 0 {
		config.Sync.SyncInterval = 2
	}
	if config.Log.Level == "" {
		config.Log.Level = "info"
	}
	if config.Log.File == "" {
		config.Log.File = "sync.log"
	}
	if config.Log.MaxSizeMB <= 0 {
		config.Log.MaxSizeMB = 10
	}
	return nil
}