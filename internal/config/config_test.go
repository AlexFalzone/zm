package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfileValidate(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		wantErr bool
	}{
		{
			name: "valid ftp profile",
			profile: Profile{
				Host:     "mainframe.example.com",
				User:     "user",
				Password: "pass",
				Protocol: "ftp",
			},
			wantErr: false,
		},
		{
			name: "valid sftp profile",
			profile: Profile{
				Host:     "mainframe.example.com",
				User:     "user",
				Password: "pass",
				Protocol: "sftp",
			},
			wantErr: false,
		},
		{
			name: "missing host",
			profile: Profile{
				User:     "user",
				Password: "pass",
				Protocol: "ftp",
			},
			wantErr: true,
		},
		{
			name: "missing user",
			profile: Profile{
				Host:     "mainframe.example.com",
				Password: "pass",
				Protocol: "ftp",
			},
			wantErr: true,
		},
		{
			name: "missing password",
			profile: Profile{
				Host:     "mainframe.example.com",
				User:     "user",
				Protocol: "ftp",
			},
			wantErr: true,
		},
		{
			name: "invalid protocol",
			profile: Profile{
				Host:     "mainframe.example.com",
				User:     "user",
				Password: "pass",
				Protocol: "telnet",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.profile.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".zmconfig")

	original := &Config{
		Profiles: map[string]*Profile{
			"test": {
				Host:     "mainframe.example.com",
				Port:     21,
				User:     "testuser",
				Password: "testpass",
				Protocol: "ftp",
				HLQ:      "TESTUSER",
				USSHome:  "/u/testuser",
			},
		},
		DefaultProfile: "test",
	}

	if err := original.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Check file permissions
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("file permissions = %o, want 0600", info.Mode().Perm())
	}

	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.DefaultProfile != original.DefaultProfile {
		t.Errorf("DefaultProfile = %q, want %q", loaded.DefaultProfile, original.DefaultProfile)
	}

	profile, err := loaded.GetProfile("test")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}

	if profile.Host != "mainframe.example.com" {
		t.Errorf("Host = %q, want mainframe.example.com", profile.Host)
	}
	if profile.User != "testuser" {
		t.Errorf("User = %q, want testuser", profile.User)
	}
}

func TestConfigGetProfile(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]*Profile{
			"prod": {Host: "prod.example.com"},
			"dev":  {Host: "dev.example.com"},
		},
		DefaultProfile: "prod",
	}

	t.Run("get by name", func(t *testing.T) {
		p, err := cfg.GetProfile("dev")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if p.Host != "dev.example.com" {
			t.Errorf("Host = %q, want dev.example.com", p.Host)
		}
	})

	t.Run("get default", func(t *testing.T) {
		p, err := cfg.GetProfile("")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if p.Host != "prod.example.com" {
			t.Errorf("Host = %q, want prod.example.com", p.Host)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := cfg.GetProfile("nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent profile")
		}
	})
}

func TestLoadDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".zmconfig")

	// Config without port and protocol
	content := `profiles:
  test:
    host: mainframe.example.com
    user: user
    password: pass
default_profile: test
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	profile, _ := cfg.GetProfile("test")
	if profile.Port != DefaultPort {
		t.Errorf("Port = %d, want %d", profile.Port, DefaultPort)
	}
	if profile.Protocol != DefaultProtocol {
		t.Errorf("Protocol = %q, want %q", profile.Protocol, DefaultProtocol)
	}
}
