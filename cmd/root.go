package cmd

import (
	"fmt"
	"os"

	"zm/internal/config"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	profile string
	cfg     *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "zm",
	Short: "z/OS Mainframe CLI tool",
	Long:  `zm is a simple CLI tool for working with z/OS mainframes.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "setup" {
			return nil
		}

		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if profile != "" {
			cfg.DefaultProfile = profile
		}

		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.zmconfig)")
	rootCmd.PersistentFlags().StringVarP(&profile, "profile", "p", "", "profile to use (overrides default)")
}

func GetConfig() *config.Config {
	return cfg
}

func GetCurrentProfile() (*config.Profile, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config not loaded")
	}
	return cfg.GetProfile(cfg.DefaultProfile)
}
