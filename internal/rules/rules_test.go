package rules

import (
	"testing"

	"github.com/vitorhugo-java/organizerv2/internal/config"
)

func defaultClassifier() *Classifier {
	cfg := config.Default()
	return NewClassifier(cfg.Rules, cfg.IgnoreExtensions, cfg.FallbackCategory)
}

func TestClassifyKnown(t *testing.T) {
	c := defaultClassifier()
	cat, ignored := c.Classify("photo.jpg")
	if ignored {
		t.Error("jpg should not be ignored")
	}
	if cat != "Image" {
		t.Errorf("expected Image, got %s", cat)
	}
}

func TestClassifyUppercase(t *testing.T) {
	c := defaultClassifier()
	cat, ignored := c.Classify("setup.EXE")
	if ignored {
		t.Fatal("exe should not be ignored")
	}
	if cat != "Executables" {
		t.Errorf("expected Executables, got %s", cat)
	}
}

func TestClassifyIgnoredTmp(t *testing.T) {
	c := defaultClassifier()
	_, ignored := c.Classify("partial.tmp")
	if !ignored {
		t.Error(".tmp should be ignored")
	}
}

func TestClassifyIgnoredQBittorrent(t *testing.T) {
	c := defaultClassifier()
	_, ignored := c.Classify("ubuntu.iso.!qB")
	if !ignored {
		t.Error(".!qB should be ignored")
	}
	_, ignored2 := c.Classify("ubuntu.iso.!qb")
	if !ignored2 {
		t.Error(".!qb should be ignored")
	}
}

func TestClassifyFallback(t *testing.T) {
	c := defaultClassifier()
	cat, ignored := c.Classify("mystery.xyz")
	if ignored {
		t.Error("unknown ext should not be ignored")
	}
	if cat != "Others" {
		t.Errorf("expected Others, got %s", cat)
	}
}

func TestClassifyTorrent(t *testing.T) {
	c := defaultClassifier()
	cat, _ := c.Classify("ubuntu.torrent")
	if cat != "Torrent" {
		t.Errorf("expected Torrent, got %s", cat)
	}
}

func TestAddRule(t *testing.T) {
	c := defaultClassifier()
	c.AddRule("CAD", []string{".dwg", ".dxf"})
	cat, _ := c.Classify("drawing.DWG")
	if cat != "CAD" {
		t.Errorf("expected CAD, got %s", cat)
	}
}

func TestRemoveExt(t *testing.T) {
	c := defaultClassifier()
	c.RemoveExt(".jpg")
	cat, _ := c.Classify("photo.jpg")
	if cat != "Others" {
		t.Errorf("after removal expected Others, got %s", cat)
	}
}

func TestCategories(t *testing.T) {
	c := defaultClassifier()
	cats := c.Categories()
	if len(cats) == 0 {
		t.Error("expected non-empty categories")
	}
	// Must be sorted
	for i := 1; i < len(cats); i++ {
		if cats[i] < cats[i-1] {
			t.Errorf("categories not sorted: %v", cats)
		}
	}
}
