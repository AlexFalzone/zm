package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigFile = ".zmconfig"
	DefaultProtocol   = "zosmf"
)

type Profile struct {
	Host          string        `yaml:"host"`
	Port          int           `yaml:"port"`
	User          string        `yaml:"user"`
	Password      string        `yaml:"password"`
	Protocol      string        `yaml:"protocol"` // zosmf, ftp, ssh
	KeyPath       string        `yaml:"key_path,omitempty"`
	HLQ           string        `yaml:"hlq"`
	USSHome       string        `yaml:"uss_home"`
	Encoding      string        `yaml:"encoding,omitempty"`       // "ascii", "ebcdic", "" (auto-detect)
	TLSVerify     bool          `yaml:"tls_verify,omitempty"`     // verify TLS certificates (default false)
	CACertPath    string        `yaml:"ca_cert_path,omitempty"`   // path to CA cert for TLS verification
	RetryAttempts int           `yaml:"retry_attempts,omitempty"` // number of retry attempts (0 = no retry)
	RetryDelay    time.Duration `yaml:"retry_delay,omitempty"`    // initial delay between retries (default 1s)
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

	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s\nRun 'zm config setup' to create one", path)
		}
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}
	if stat.Mode().Perm()&0077 != 0 {
		fmt.Fprintf(os.Stderr, "warning: config file %s has insecure permissions %04o, should be 0600\n", path, stat.Mode().Perm())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config file: %w", err)
	}

	for name, p := range cfg.Profiles {
		if p.Protocol == "" {
			p.Protocol = DefaultProtocol
		}
		if p.Port == 0 {
			p.Port = DefaultPortForProtocol(p.Protocol)
		}
		if err := p.Validate(); err != nil {
			return nil, fmt.Errorf("profile '%s': %w", name, err)
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
	if p.Protocol == "ssh" {
		if p.Password == "" && p.KeyPath == "" {
			return fmt.Errorf("password or key_path is required for SSH")
		}
	} else if p.Password == "" {
		return fmt.Errorf("password is required")
	}
	if p.Protocol != "zosmf" && p.Protocol != "ftp" && p.Protocol != "ssh" {
		return fmt.Errorf("protocol must be 'zosmf', 'ftp', or 'ssh'")
	}
	return nil
}

func DefaultPortForProtocol(protocol string) int {
	switch protocol {
	case "zosmf":
		return 443
	case "ssh":
		return 22
	default:
		return 21
	}
}
