package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vitorhugo-java/organizerv2/internal/notifier"
	"github.com/vitorhugo-java/organizerv2/internal/organizer"
	"github.com/vitorhugo-java/organizerv2/internal/rules"
)

var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "One-shot scan and organize a directory",
	Long: `Scan a directory once and move files into category subfolders.
If no path argument is given, all watch_paths from the config are scanned.

Use --dry-run to preview what would happen without moving any files.`,
	RunE: runScan,
}

var dryRun bool

func init() {
	scanCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would happen without moving files")
	rootCmd.AddCommand(scanCmd)
}

func runScan(cmd *cobra.Command, args []string) error {
	clf := rules.NewClassifier(cfg.Rules, cfg.IgnoreExtensions, cfg.FallbackCategory)

	var n notifier.Notifier = notifier.NoopNotifier{}
	if !dryRun {
		n = notifier.New(cfg.Notifications)
		defer n.Close()
	}

	org := organizer.New(cfg, clf, n)

	paths := args
	if len(paths) == 0 {
		for _, wp := range cfg.WatchPaths {
			paths = append(paths, wp.Path)
		}
	}
	if len(paths) == 0 {
		return fmt.Errorf("no paths to scan; specify a path or configure watch_paths")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tFILE\tCATEGORY\tDESTINATION")
	fmt.Fprintln(w, "------\t----\t--------\t-----------")

	var totalMoved, totalSkipped, totalErrors int

	for _, scanPath := range paths {
		if dryRun {
			printDryRun(w, scanPath, clf, cfg.FallbackCategory, &totalMoved, &totalSkipped)
		} else {
			results := org.ScanDir(scanPath)
			for _, r := range results {
				printResult(w, r, &totalMoved, &totalSkipped, &totalErrors)
			}
		}
	}

	w.Flush()
	fmt.Printf("\nSummary: %d moved, %d skipped, %d errors\n", totalMoved, totalSkipped, totalErrors)
	return nil
}

func printResult(w *tabwriter.Writer, r organizer.MoveResult, moved, skipped, errs *int) {
	switch {
	case r.Err != nil:
		fmt.Fprintf(w, "ERROR\t%s\t\t%v\n", filepath.Base(r.Source), r.Err)
		*errs++
	case r.Skipped:
		fmt.Fprintf(w, "SKIP\t%s\t\t%s\n", filepath.Base(r.Source), r.SkipReason)
		*skipped++
	default:
		fmt.Fprintf(w, "MOVED\t%s\t%s\t%s\n", filepath.Base(r.Source), r.Category, r.Destination)
		*moved++
	}
}

func printDryRun(w *tabwriter.Writer, dir string, clf *rules.Classifier, fallback string, moved, skipped *int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(w, "ERROR\t%s\t\t%v\n", dir, err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		cat, ignored := clf.Classify(entry.Name())
		if ignored {
			fmt.Fprintf(w, "SKIP\t%s\t\tignored extension\n", entry.Name())
			*skipped++
		} else {
			fmt.Fprintf(w, "WOULD MOVE\t%s\t%s\t%s/%s/%s\n",
				entry.Name(), cat, dir, cat, entry.Name())
			*moved++
		}
	}
}
