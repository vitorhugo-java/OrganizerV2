package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vitorhugo-java/organizerv2/internal/config"
)

var (
	cfgFile string
	cfg     *config.Config

	// version is injected at build time via -ldflags.
	version = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "organizer",
	Short: "OrganizerV2 — automatic cross-platform file organizer",
	Long: `OrganizerV2 watches directories for new files and sorts them into
category subfolders based on their file extension.

Run 'organizer start' to launch the file watcher daemon.
Run 'organizer scan'  to perform a one-shot scan and organize.
Run 'organizer config init' to generate a default configuration file.`,
	SilenceUsage: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}

// Execute is the entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	defaultCfg, _ := defaultConfigPath()
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", defaultCfg, "config file path")
	rootCmd.PersistentFlags().String("log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.AddCommand(versionCmd)
	cobra.OnInitialize(loadConfig)
}

func loadConfig() {
	var err error
	cfg, err = config.Load(cfgFile)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}
}

func defaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml", err
	}
	return filepath.Join(home, ".config", "organizerv2", "config.yaml"), nil
}
