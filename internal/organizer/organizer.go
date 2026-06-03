package organizer

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/vitorhugo-java/organizerv2/internal/config"
	"github.com/vitorhugo-java/organizerv2/internal/notifier"
	"github.com/vitorhugo-java/organizerv2/internal/pathutil"
	"github.com/vitorhugo-java/organizerv2/internal/rules"
)

// MoveResult describes the outcome of processing a single file.
type MoveResult struct {
	Source      string
	Destination string
	Category    string
	Skipped     bool
	SkipReason  string
	Err         error
}

// Organizer classifies and moves files according to configured rules.
type Organizer struct {
	cfg        *config.Config
	classifier *rules.Classifier
	notifier   notifier.Notifier
}

// New creates an Organizer wired to the given config, classifier and notifier.
func New(cfg *config.Config, clf *rules.Classifier, n notifier.Notifier) *Organizer {
	return &Organizer{cfg: cfg, classifier: clf, notifier: n}
}

// ProcessFile classifies and moves a single file. It is safe to call
// concurrently from the watcher goroutine.
func (o *Organizer) ProcessFile(srcPath string) MoveResult {
	info, err := os.Stat(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return MoveResult{Source: srcPath, Skipped: true, SkipReason: "not found"}
		}
		return MoveResult{Source: srcPath, Err: fmt.Errorf("stat: %w", err)}
	}
	if !info.Mode().IsRegular() {
		return MoveResult{Source: srcPath, Skipped: true, SkipReason: "not a regular file"}
	}

	category, ignored := o.classifier.Classify(filepath.Base(srcPath))
	if ignored {
		return MoveResult{Source: srcPath, Skipped: true, SkipReason: "ignored extension"}
	}

	targetBase := o.resolveTargetBase(srcPath)
	destDir := filepath.Join(targetBase, category)
	if err := pathutil.EnsureDir(destDir); err != nil {
		return MoveResult{Source: srcPath, Err: fmt.Errorf("mkdir %s: %w", destDir, err)}
	}

	raw := pathutil.ResolveDestination(targetBase, category, filepath.Base(srcPath))
	dest, err := pathutil.ResolveDuplicate(raw)
	if err != nil {
		return MoveResult{Source: srcPath, Err: err}
	}

	if err := moveFile(srcPath, dest); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return MoveResult{Source: srcPath, Skipped: true, SkipReason: "permission denied (file in use)"}
		}
		return MoveResult{Source: srcPath, Err: fmt.Errorf("move: %w", err)}
	}

	result := MoveResult{
		Source:      srcPath,
		Destination: dest,
		Category:    category,
	}

	go func() {
		if err := o.notifier.Notify(notifier.FileEvent{
			Source:      srcPath,
			Destination: dest,
			Category:    category,
		}); err != nil {
			log.Printf("[organizer] notification error: %v", err)
		}
	}()

	return result
}

// ScanDir performs a one-shot scan of dirPath, processing every regular file
// found at the top level (no recursion into subdirectories).
func (o *Organizer) ScanDir(dirPath string) []MoveResult {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return []MoveResult{{Source: dirPath, Err: fmt.Errorf("readdir: %w", err)}}
	}
	var results []MoveResult
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		results = append(results, o.ProcessFile(filepath.Join(dirPath, entry.Name())))
	}
	return results
}

// resolveTargetBase finds which configured WatchPath contains srcPath and
// returns its TargetBase. Falls back to the directory of srcPath.
func (o *Organizer) resolveTargetBase(srcPath string) string {
	abs, err := filepath.Abs(srcPath)
	if err != nil {
		return filepath.Dir(srcPath)
	}
	for _, wp := range o.cfg.WatchPaths {
		watchAbs, err := filepath.Abs(wp.Path)
		if err != nil {
			continue
		}
		if strings.HasPrefix(abs, watchAbs+string(filepath.Separator)) ||
			abs == watchAbs {
			return wp.TargetBase
		}
	}
	return filepath.Dir(srcPath)
}

// moveFile moves src to dst, using os.Rename first. If that fails due to a
// cross-device link error it falls back to copy-then-delete.
func moveFile(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	if !isCrossDevice(err) {
		return err
	}
	return copyAndDelete(src, dst)
}

func copyAndDelete(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return fmt.Errorf("copy: %w", err)
	}
	if err := out.Close(); err != nil {
		os.Remove(dst)
		return fmt.Errorf("close dest: %w", err)
	}
	return os.Remove(src)
}
