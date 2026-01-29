package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigFile = ".zmconfig"
	DefaultPort       = 21
	DefaultProtocol   = "ftp"
)

type Profile struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Protocol string `yaml:"protocol"` // ftp, sftp
	HLQ      string `yaml:"hlq"`
	USSHome  string `yaml:"uss_home"`
}

type Config struct {
	Profiles       map[string]*Profile `yaml:"profiles"`
	DefaultProfile string              `yaml:"default_profile"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot find home directory: %w", err)
		}
		path = filepath.Join(home, DefaultConfigFile)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s\nRun 'zm config setup' to create one", path)
		}
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config file: %w", err)
	}

	for name, p := range cfg.Profiles {
		if p.Port == 0 {
			p.Port = DefaultPort
		}
		if p.Protocol == "" {
			p.Protocol = DefaultProtocol
		}
		cfg.Profiles[name] = p
	}

	return &cfg, nil
}

func (c *Config) Save(path string) error {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot find home directory: %w", err)
		}
		path = filepath.Join(home, DefaultConfigFile)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}

	// 0600: owner read/write only (contains password)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("cannot write config file: %w", err)
	}

	return nil
}

func (c *Config) GetProfile(name string) (*Profile, error) {
	if name == "" {
		name = c.DefaultProfile
	}
	if name == "" {
		return nil, fmt.Errorf("no profile specified and no default profile set")
	}

	p, ok := c.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile '%s' not found", name)
	}

	return p, nil
}

func (p *Profile) Validate() error {
	if p.Host == "" {
		return fmt.Errorf("host is required")
	}
	if p.User == "" {
		return fmt.Errorf("user is required")
	}
	if p.Password == "" {
		return fmt.Errorf("password is required")
	}
	if p.Protocol != "ftp" && p.Protocol != "sftp" {
		return fmt.Errorf("protocol must be 'ftp' or 'sftp'")
	}
	return nil
}
