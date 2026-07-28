package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestActiveProjectBrandingHasNoLegacySelectors(t *testing.T) {
	t.Helper()
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	bannedWords := []string{
		"starter",
		"edition",
	}
	bannedSelectors := []string{
		"--edition",
		"framework_version",
		"--framework-version",
		"--framework-replace",
		"github.com/Alfian57/gin-kit/framework",
		"templates/framework",
		"resource/framework",
		"framework-only",
		"GIN_KIT_EDITION",
	}
	extensions := map[string]bool{
		".go": true, ".md": true, ".mdx": true, ".tmpl": true,
		".yml": true, ".yaml": true, ".sh": true, ".mjs": true,
	}

	err := filepath.WalkDir(repoRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, "_test.go") || !extensions[filepath.Ext(path)] {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, term := range bannedWords {
			if regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(term) + `\b`).Match(content) {
				t.Errorf("legacy project selector %q remains in %s", term, path)
			}
		}
		for _, term := range bannedSelectors {
			if strings.Contains(strings.ToLower(string(content)), strings.ToLower(term)) {
				t.Errorf("legacy project selector %q remains in %s", term, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
