package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"zm/internal/config"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage zm configuration",
}

var configSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Create or update configuration",
	Long:  `Interactive setup to create or update the zm configuration file.`,
	RunE:  runConfigSetup,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSetupCmd)
}

func runConfigSetup(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("zm Configuration Setup")
	fmt.Println("======================")
	fmt.Println()

	// Profile name
	profileName := prompt(reader, "Profile name", "default")

	// Host
	host := prompt(reader, "Mainframe host", "")
	if host == "" {
		return fmt.Errorf("host is required")
	}

	// Port
	portStr := prompt(reader, "Port", "21")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid port: %s", portStr)
	}

	// User
	user := prompt(reader, "Username", "")
	if user == "" {
		return fmt.Errorf("username is required")
	}

	// Password
	password := promptPassword(reader, "Password")
	if password == "" {
		return fmt.Errorf("password is required")
	}

	// Protocol
	protocol := prompt(reader, "Protocol (ftp/sftp)", "sftp")
	if protocol != "ftp" && protocol != "sftp" {
		return fmt.Errorf("protocol must be 'ftp' or 'sftp'")
	}

	// HLQ
	hlq := prompt(reader, "High Level Qualifier (e.g., USERNAME)", strings.ToUpper(user))

	// USS Home
	ussHome := prompt(reader, "USS Home directory", fmt.Sprintf("/u/%s", strings.ToLower(user)))

	// Create config
	profile := &config.Profile{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Protocol: protocol,
		HLQ:      hlq,
		USSHome:  ussHome,
	}

	// Load existing config or create new
	var cfg *config.Config
	cfg, err = config.Load(cfgFile)
	if err != nil {
		// Create new config
		cfg = &config.Config{
			Profiles:       make(map[string]*config.Profile),
			DefaultProfile: profileName,
		}
	}

	cfg.Profiles[profileName] = profile

	// Ask if this should be the default profile
	if len(cfg.Profiles) > 1 {
		setDefault := prompt(reader, fmt.Sprintf("Set '%s' as default profile? (y/n)", profileName), "y")
		if strings.ToLower(setDefault) == "y" {
			cfg.DefaultProfile = profileName
		}
	} else {
		cfg.DefaultProfile = profileName
	}

	// Save config
	if err := cfg.Save(cfgFile); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	fmt.Println("Configuration saved successfully!")
	fmt.Printf("Config file: ~/.zmconfig\n")
	fmt.Printf("Default profile: %s\n", cfg.DefaultProfile)

	return nil
}

func prompt(reader *bufio.Reader, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("%s: ", label)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultVal
	}
	return input
}

func promptPassword(reader *bufio.Reader, label string) string {
	fmt.Printf("%s: ", label)
	// Note: In a real implementation, we'd use term.ReadPassword for hidden input
	// For now, just read normally (password will be visible)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}
