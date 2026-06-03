package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vitorhugo-java/organizerv2/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a default config file",
	Long:  `Write the default configuration to the config file path (--config flag). Warns if the file already exists.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := os.Stat(cfgFile); err == nil {
			fmt.Printf("Config file already exists at %s\nUse --config to specify a different path.\n", cfgFile)
			return nil
		}
		if err := config.Save(config.Default(), cfgFile); err != nil {
			return err
		}
		fmt.Printf("Config written to %s\n", cfgFile)
		return nil
	},
}

var rulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "Manage file classification rules",
}

var rulesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List current classification rules",
	Run: func(cmd *cobra.Command, args []string) {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "CATEGORY\tEXTENSIONS")
		fmt.Fprintln(w, "--------\t----------")
		rules := cfg.Rules
		sort.Slice(rules, func(i, j int) bool { return rules[i].Category < rules[j].Category })
		for _, r := range rules {
			fmt.Fprintf(w, "%s\t%s\n", r.Category, strings.Join(r.Extensions, ", "))
		}
		w.Flush()
		fmt.Printf("\nIgnored: %s\n", strings.Join(cfg.IgnoreExtensions, ", "))
		fmt.Printf("Fallback category: %s\n", cfg.FallbackCategory)
	},
}

var (
	addCategory string
	addExt      string
	removeExt   string
)

var rulesAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add an extension to a category",
	Example: `  organizer config rules add --category Image --ext .webp
  organizer config rules add --category Custom --ext .abc`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if addCategory == "" || addExt == "" {
			return fmt.Errorf("--category and --ext are required")
		}
		ext := strings.ToLower(addExt)
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		for i, r := range cfg.Rules {
			if strings.EqualFold(r.Category, addCategory) {
				for _, e := range r.Extensions {
					if e == ext {
						fmt.Printf("Extension %s already in category %s\n", ext, r.Category)
						return nil
					}
				}
				cfg.Rules[i].Extensions = append(r.Extensions, ext)
				return config.Save(cfg, cfgFile)
			}
		}
		// New category.
		cfg.Rules = append(cfg.Rules, config.Rule{Category: addCategory, Extensions: []string{ext}})
		if err := config.Save(cfg, cfgFile); err != nil {
			return err
		}
		fmt.Printf("Added %s → %s\n", ext, addCategory)
		return nil
	},
}

var rulesRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove an extension from all categories",
	Example: `  organizer config rules remove --ext .webp`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if removeExt == "" {
			return fmt.Errorf("--ext is required")
		}
		ext := strings.ToLower(removeExt)
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		found := false
		for i, r := range cfg.Rules {
			var kept []string
			for _, e := range r.Extensions {
				if e != ext {
					kept = append(kept, e)
				} else {
					found = true
				}
			}
			cfg.Rules[i].Extensions = kept
		}
		if !found {
			fmt.Printf("Extension %s not found in any category\n", ext)
			return nil
		}
		if err := config.Save(cfg, cfgFile); err != nil {
			return err
		}
		fmt.Printf("Removed %s\n", ext)
		return nil
	},
}

func init() {
	rulesAddCmd.Flags().StringVar(&addCategory, "category", "", "target category name")
	rulesAddCmd.Flags().StringVar(&addExt, "ext", "", "file extension (e.g. .webp)")
	rulesRemoveCmd.Flags().StringVar(&removeExt, "ext", "", "file extension to remove")

	rulesCmd.AddCommand(rulesListCmd, rulesAddCmd, rulesRemoveCmd)
	configCmd.AddCommand(configInitCmd, rulesCmd)
	rootCmd.AddCommand(configCmd)
}
