package rules

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/vitorhugo-java/organizerv2/internal/config"
)

// Classifier maps file extensions to destination categories.
// It is safe for concurrent use.
type Classifier struct {
	mu           sync.RWMutex
	extensionMap map[string]string // lowercase ext -> category
	ignoreSet    map[string]struct{}
	fallback     string
}

// NewClassifier builds a Classifier from config rules.
func NewClassifier(rules []config.Rule, ignoreExts []string, fallback string) *Classifier {
	c := &Classifier{
		extensionMap: make(map[string]string),
		ignoreSet:    make(map[string]struct{}),
		fallback:     fallback,
	}
	for _, r := range rules {
		for _, ext := range r.Extensions {
			c.extensionMap[strings.ToLower(ext)] = r.Category
		}
	}
	for _, ext := range ignoreExts {
		// Store both original casing and lowercased so .!qB and .!qb both match.
		c.ignoreSet[ext] = struct{}{}
		c.ignoreSet[strings.ToLower(ext)] = struct{}{}
	}
	return c
}

// Classify returns the destination category for a filename and whether the file
// should be ignored entirely. ignored=true means the file must not be moved.
func (c *Classifier) Classify(filename string) (category string, ignored bool) {
	ext := filepath.Ext(filename)
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check ignore set with both original casing and lowercased.
	if _, ok := c.ignoreSet[ext]; ok {
		return "", true
	}
	if _, ok := c.ignoreSet[strings.ToLower(ext)]; ok {
		return "", true
	}

	if cat, ok := c.extensionMap[strings.ToLower(ext)]; ok {
		return cat, false
	}
	return c.fallback, false
}

// Categories returns a sorted slice of all known category names.
func (c *Classifier) Categories() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	seen := make(map[string]struct{})
	for _, cat := range c.extensionMap {
		seen[cat] = struct{}{}
	}
	cats := make([]string, 0, len(seen))
	for cat := range seen {
		cats = append(cats, cat)
	}
	sort.Strings(cats)
	return cats
}

// AddRule adds or overwrites mappings for a category. Extensions are normalized
// to lowercase.
func (c *Classifier) AddRule(category string, exts []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, ext := range exts {
		c.extensionMap[strings.ToLower(ext)] = category
	}
}

// RemoveExt removes an extension mapping so it falls back to the fallback category.
func (c *Classifier) RemoveExt(ext string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.extensionMap, strings.ToLower(ext))
}
