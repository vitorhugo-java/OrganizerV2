package watcher

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vitorhugo-java/organizerv2/internal/config"
	"github.com/vitorhugo-java/organizerv2/internal/organizer"
)

const debounceDelay = 500 * time.Millisecond

// partialExts are extensions used by browsers for in-progress downloads.
// Files with these extensions are skipped until the browser renames them.
var partialExts = map[string]struct{}{
	".crdownload": {}, // Chrome
	".part":       {}, // Firefox
	".download":   {}, // Safari / generic
	".tmp":        {},
	".opdownload": {}, // Opera
}

// sizeStabilityDelay is how long a file's size must remain unchanged before
// it is considered fully written and safe to move.
const sizeStabilityDelay = 2 * time.Second

// Watcher watches configured directories for new files and triggers the
// organizer on each new or completed file.
type Watcher struct {
	fsw       *fsnotify.Watcher
	org       *organizer.Organizer
	cfg       *config.Config
	timers    map[string]*time.Timer
	timersMu  sync.Mutex
	// categoryDirs is the set of category subdirectory prefixes we created.
	// Events for paths inside these dirs are ignored to prevent reprocessing.
	categoryDirs map[string]struct{}
}

// New creates a Watcher for all paths in cfg.WatchPaths.
func New(cfg *config.Config, org *organizer.Organizer) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	for _, wp := range cfg.WatchPaths {
		if err := fsw.Add(wp.Path); err != nil {
			fsw.Close()
			return nil, err
		}
		log.Printf("[watcher] watching %s", wp.Path)
	}
	return &Watcher{
		fsw:          fsw,
		org:          org,
		cfg:          cfg,
		timers:       make(map[string]*time.Timer),
		categoryDirs: make(map[string]struct{}),
	}, nil
}

// Start begins watching and blocks until ctx is cancelled.
func (w *Watcher) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
				w.schedule(event.Name)
			}
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			log.Printf("[watcher] error: %v", err)
		}
	}
}

// Stop closes the underlying fsnotify watcher.
func (w *Watcher) Stop() {
	w.fsw.Close()
}

func (w *Watcher) schedule(path string) {
	// Skip files inside category subdirectories to prevent reprocessing loops.
	if w.isCategoryPath(path) {
		return
	}
	// Ignore partial-download extensions immediately; the browser will rename
	// the file once the download finishes, which will trigger a new event.
	if _, partial := partialExts[strings.ToLower(filepath.Ext(path))]; partial {
		return
	}
	w.timersMu.Lock()
	defer w.timersMu.Unlock()
	if t, ok := w.timers[path]; ok {
		t.Reset(debounceDelay)
		return
	}
	w.timers[path] = time.AfterFunc(debounceDelay, func() {
		w.timersMu.Lock()
		delete(w.timers, path)
		w.timersMu.Unlock()

		if !w.isFileStable(path) {
			// File is still being written; reschedule and wait.
			w.timersMu.Lock()
			w.timers[path] = time.AfterFunc(sizeStabilityDelay, func() {
				w.timersMu.Lock()
				delete(w.timers, path)
				w.timersMu.Unlock()
				w.schedule(path)
			})
			w.timersMu.Unlock()
			return
		}

		result := w.org.ProcessFile(path)
		if result.Err != nil {
			log.Printf("[watcher] error processing %s: %v", path, result.Err)
		} else if !result.Skipped {
			// Track the category directory so future events inside it are ignored.
			w.timersMu.Lock()
			w.categoryDirs[filepath.Dir(result.Destination)] = struct{}{}
			w.timersMu.Unlock()
			log.Printf("[watcher] moved %s → %s", filepath.Base(path), result.Category)
		}
	})
}

// isFileStable returns true if the file's size has not changed over
// sizeStabilityDelay, indicating the write is complete.
func (w *Watcher) isFileStable(path string) bool {
	info1, err := os.Stat(path)
	if err != nil {
		return false
	}
	time.Sleep(sizeStabilityDelay)
	info2, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info1.Size() == info2.Size()
}

// isCategoryPath returns true if path is inside a known category subdirectory.
func (w *Watcher) isCategoryPath(path string) bool {
	w.timersMu.Lock()
	defer w.timersMu.Unlock()
	dir := filepath.Dir(path)
	if _, ok := w.categoryDirs[dir]; ok {
		return true
	}
	// Also check statically: if the parent dir name matches a known category.
	for _, wp := range w.cfg.WatchPaths {
		watchAbs, err := filepath.Abs(wp.Path)
		if err != nil {
			continue
		}
		dirAbs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		// If the file's parent is a direct child of the watch path, it may be a
		// category folder. We check by verifying it is a subdirectory one level deep.
		parentOfParent := filepath.Dir(dirAbs)
		if strings.EqualFold(parentOfParent, watchAbs) {
			return true
		}
	}
	return false
}
