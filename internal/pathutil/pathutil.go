package pathutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveDestination returns the target path for a file inside a category
// subfolder of targetBase.
func ResolveDestination(targetBase, category, filename string) string {
	return filepath.Join(targetBase, category, filename)
}

// ResolveDuplicate returns a path that does not yet exist on disk. If destPath
// is free it is returned as-is. If it already exists the stem gets " (N)"
// appended until a free name is found (up to 1000 attempts).
func ResolveDuplicate(destPath string) (string, error) {
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return destPath, nil
	}
	dir := filepath.Dir(destPath)
	base := filepath.Base(destPath)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	for n := 2; n <= 1000; n++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, n, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find a free filename for %s after 1000 attempts", destPath)
}

// EnsureDir creates dir and all parent directories with mode 0755.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

// ExpandHome replaces a leading "~/" with the current user's home directory.
func ExpandHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") && path != "~" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home dir: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}
