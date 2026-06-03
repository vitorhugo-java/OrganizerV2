package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/vitorhugo-java/organizerv2/internal/notifier"
	"github.com/vitorhugo-java/organizerv2/internal/organizer"
	"github.com/vitorhugo-java/organizerv2/internal/rules"
	"github.com/vitorhugo-java/organizerv2/internal/watcher"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start watching configured directories",
	Long:  `Start the file watcher daemon. The process runs until interrupted with Ctrl+C or SIGTERM.`,
	RunE:  runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	if len(cfg.WatchPaths) == 0 {
		return fmt.Errorf("no watch_paths configured; run 'organizer config init' to create a config file")
	}

	clf := rules.NewClassifier(cfg.Rules, cfg.IgnoreExtensions, cfg.FallbackCategory)
	n := notifier.New(cfg.Notifications)
	defer n.Close()

	org := organizer.New(cfg, clf, n)

	w, err := watcher.New(cfg, org)
	if err != nil {
		return fmt.Errorf("creating watcher: %w", err)
	}
	defer w.Stop()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("[start] organizer running — watching %d director(ies). Press Ctrl+C to stop.", len(cfg.WatchPaths))
	w.Start(ctx)
	log.Println("[start] shutting down")
	return nil
}
