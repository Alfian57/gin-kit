package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// baselineFile records the checksum every vendored platform file had when the
// CLI last wrote it. It lets gin-kit upgrade tell a stale vendored copy (safe
// to update) from a local edit (never overwritten without --force). The
// manifest itself stays untouched: released CLIs decode .gin-kit.yaml with
// KnownFields(true), so new manifest fields would break them.
const baselineFile = ".gin-kit.sum"

// platformPrefix is the project-relative root of the vendored runtime that
// gin-kit upgrade manages in standalone projects.
const platformPrefix = "internal/platform/"

// upgradeStatus defines an implementation type used by this package.
type upgradeStatus string

const (
	// statusUpToDate define package-level implementation state.
	statusUpToDate upgradeStatus = "up-to-date"
	// statusOutdated define package-level implementation state.
	statusOutdated upgradeStatus = "outdated"
	// statusModified define package-level implementation state.
	statusModified upgradeStatus = "modified"
	// statusDiffers define package-level implementation state.
	statusDiffers upgradeStatus = "differs"
	// statusMissing define package-level implementation state.
	statusMissing upgradeStatus = "missing"
	// statusUnmanaged define package-level implementation state.
	statusUnmanaged upgradeStatus = "unmanaged"
)

// upgradeEntry is the per-file result of comparing the current CLI's rendered
// platform templates with the project on disk.
type upgradeEntry struct {
	Path     string        // project-relative slash path
	Status   upgradeStatus // classification, see upgradePlan
	Rendered []byte        // desired content (gofmt-normalized); nil for unmanaged files
	Disk     []byte        // current on-disk content; nil for missing files
}

// upgradeCommand performs this package operation.
func upgradeCommand() *cobra.Command {
	var applyChanges, showDiff, force bool
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Update the vendored internal/platform code of a standalone project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, m, err := projectRoot()
			if err != nil {
				return err
			}
			if m.ProjectType != "standalone" {
				return diagnostic("upgrade_project_type_unsupported", "upgrade platform code", rootDir,
					errors.New("gin-kit upgrade manages the vendored internal/platform code of standalone projects only"),
					"Runtime projects upgrade the versioned runtime instead: go get github.com/Alfian57/gin-kit@vX.Y.Z && go mod tidy.")
			}
			entries, err := upgradePlan(rootDir, m)
			if err != nil {
				return err
			}
			if applyChanges {
				applied, skipped, err := applyUpgrade(rootDir, entries, force)
				for _, path := range applied {
					fmt.Println("updated", path)
				}
				for _, path := range skipped {
					fmt.Println("skipped", path, "(local changes; re-run with --force to overwrite)")
				}
				if err != nil {
					return err
				}
				fmt.Printf("Applied %d update(s), skipped %d; refreshed %s.\n", len(applied), len(skipped), baselineFile)
				return nil
			}
			counts := map[upgradeStatus]int{}
			for _, entry := range entries {
				counts[entry.Status]++
				if entry.Status == statusUpToDate {
					continue
				}
				fmt.Printf("%-9s  %s\n", entry.Status, entry.Path)
				if showDiff && entry.Status != statusUnmanaged {
					fmt.Print(unifiedDiff(entry.Path, entry.Disk, entry.Rendered))
				}
			}
			fmt.Printf("%d platform files: %d up-to-date, %d outdated, %d modified, %d differ, %d missing, %d unmanaged.\n",
				len(entries), counts[statusUpToDate], counts[statusOutdated], counts[statusModified],
				counts[statusDiffers], counts[statusMissing], counts[statusUnmanaged])
			if counts[statusOutdated]+counts[statusMissing] > 0 {
				fmt.Println("Run gin-kit upgrade --apply to write the safe updates.")
			}
			if counts[statusModified]+counts[statusDiffers] > 0 {
				fmt.Println("Modified and unverified files are skipped by --apply; inspect them with --diff and overwrite with --apply --force.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&applyChanges, "apply", false, "write safe updates (outdated and missing files) to disk")
	cmd.Flags().BoolVar(&showDiff, "diff", false, "print a unified diff for every file that differs")
	cmd.Flags().BoolVar(&force, "force", false, "with --apply, also overwrite modified and unverified files")
	return cmd
}

// upgradePlan renders the platform templates the manifest selects and
// classifies every file:
//
//   - up-to-date: disk matches the render (gofmt-normalized comparison)
//   - outdated:   disk differs but matches the recorded baseline — a stale
//     vendored copy that is safe to update automatically
//   - modified:   disk differs from both render and baseline — local edits,
//     skipped unless --force
//   - differs:    disk differs and no baseline entry exists — unverifiable,
//     skipped unless --force
//   - missing:    the rendered file does not exist on disk
//   - unmanaged:  an on-disk file under internal/platform the current
//     templates do not render — never touched
func upgradePlan(rootDir string, m Manifest) ([]upgradeEntry, error) {
	rendered, err := renderScaffoldTree(m, scaffoldOptions{}, func(rel string) bool {
		return strings.HasPrefix(rel, platformPrefix)
	})
	if err != nil {
		return nil, err
	}
	baseline, err := readBaseline(rootDir)
	if err != nil {
		return nil, err
	}
	entries := make([]upgradeEntry, 0, len(rendered))
	for rel, content := range rendered {
		desired := normalizeGo(rel, content)
		diskPath := filepath.Join(rootDir, filepath.FromSlash(rel))
		disk, readErr := os.ReadFile(diskPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				entries = append(entries, upgradeEntry{Path: rel, Status: statusMissing, Rendered: desired})
				continue
			}
			return nil, diagnostic("upgrade_read_failed", "read platform file", diskPath, readErr, "Check file permissions.")
		}
		entry := upgradeEntry{Path: rel, Rendered: desired, Disk: disk}
		switch {
		case bytes.Equal(normalizeGo(rel, disk), desired):
			entry.Status = statusUpToDate
		case baseline[rel] == "":
			entry.Status = statusDiffers
		case baseline[rel] == baselineHash(rel, disk):
			entry.Status = statusOutdated
		default:
			entry.Status = statusModified
		}
		entries = append(entries, entry)
	}
	platformDir := filepath.Join(rootDir, filepath.FromSlash(platformPrefix))
	walkErr := filepath.WalkDir(platformDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relFromRoot, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return relErr
		}
		rel := filepath.ToSlash(relFromRoot)
		if _, ok := rendered[rel]; ok {
			return nil
		}
		entries = append(entries, upgradeEntry{Path: rel, Status: statusUnmanaged})
		return nil
	})
	if walkErr != nil && !os.IsNotExist(walkErr) {
		return nil, diagnostic("upgrade_inspection_failed", "inspect platform directory", platformDir, walkErr, "Check directory permissions.")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

// applyUpgrade writes outdated and missing files (plus modified and differing
// files when force is set) through a staging directory, then refreshes the
// baseline entry of every rendered file that now matches disk — which also
// bootstraps .gin-kit.sum for projects scaffolded before it existed.
func applyUpgrade(rootDir string, entries []upgradeEntry, force bool) (applied, skipped []string, err error) {
	var toWrite []upgradeEntry
	for _, entry := range entries {
		switch entry.Status {
		case statusOutdated, statusMissing:
			toWrite = append(toWrite, entry)
		case statusModified, statusDiffers:
			if force {
				toWrite = append(toWrite, entry)
			} else {
				skipped = append(skipped, entry.Path)
			}
		}
	}
	if len(toWrite) > 0 {
		staging, stagingErr := os.MkdirTemp(rootDir, ".gin-kit-upgrade-*")
		if stagingErr != nil {
			return nil, skipped, diagnostic("upgrade_staging_failed", "stage upgraded files", rootDir, stagingErr, "Check directory permissions and available disk space.")
		}
		defer os.RemoveAll(staging)
		for _, entry := range toWrite {
			staged := filepath.Join(staging, filepath.FromSlash(entry.Path))
			if err := os.MkdirAll(filepath.Dir(staged), 0o755); err != nil {
				return nil, skipped, diagnostic("upgrade_staging_failed", "stage upgraded files", staged, err, "Check directory permissions.")
			}
			if err := os.WriteFile(staged, entry.Rendered, 0o644); err != nil {
				return nil, skipped, diagnostic("upgrade_staging_failed", "stage upgraded files", staged, err, "Check directory permissions and available disk space.")
			}
		}
		// Every file is staged before the first one is published, so a render
		// or write failure cannot leave the project half-upgraded.
		for _, entry := range toWrite {
			target := filepath.Join(rootDir, filepath.FromSlash(entry.Path))
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return applied, skipped, diagnostic("upgrade_publish_failed", "publish upgraded files", target, err, "Check directory permissions.")
			}
			if err := os.Rename(filepath.Join(staging, filepath.FromSlash(entry.Path)), target); err != nil {
				return applied, skipped, diagnostic("upgrade_publish_failed", "publish upgraded files", target, err, "Re-run gin-kit upgrade --apply; files already published are up to date.")
			}
			applied = append(applied, entry.Path)
		}
	}
	baseline, err := readBaseline(rootDir)
	if err != nil {
		return applied, skipped, err
	}
	for _, entry := range entries {
		if entry.Status == statusUnmanaged {
			continue
		}
		disk, readErr := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(entry.Path)))
		if readErr != nil {
			continue
		}
		if bytes.Equal(normalizeGo(entry.Path, disk), entry.Rendered) {
			baseline[entry.Path] = baselineHash(entry.Path, disk)
		}
	}
	if err := writeBaseline(rootDir, baseline); err != nil {
		return applied, skipped, err
	}
	return applied, skipped, nil
}

// normalizeGo formats Go sources before comparison so gofmt version drift is
// not reported as a content change. Non-Go files and unparsable sources pass
// through unchanged.
func normalizeGo(rel string, content []byte) []byte {
	if !strings.HasSuffix(rel, ".go") {
		return content
	}
	formatted, err := format.Source(content)
	if err != nil {
		return content
	}
	return formatted
}

// hashBytes performs this package operation.
func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// baselineHash hashes the same representation used by upgrade comparisons.
// This keeps harmless Go formatting differences from looking like local edits
// when a later CLI renders otherwise changed platform code.
func baselineHash(rel string, content []byte) string {
	return hashBytes(normalizeGo(rel, content))
}

// readBaseline loads .gin-kit.sum. A missing file is not an error: projects
// scaffolded before the baseline existed simply have no entries yet.
func readBaseline(rootDir string) (map[string]string, error) {
	path := filepath.Join(rootDir, baselineFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, diagnostic("baseline_unreadable", "read upgrade baseline", path, err, "Check file permissions.")
	}
	sums := map[string]string{}
	for lineNumber, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, diagnostic("baseline_invalid", "read upgrade baseline", path,
				fmt.Errorf("line %d: expected \"<sha256> <path>\"", lineNumber+1),
				"Delete the file and run gin-kit upgrade --apply to rebuild it.")
		}
		sums[fields[1]] = fields[0]
	}
	return sums, nil
}

// writeBaseline writes .gin-kit.sum as sorted "sha256hex  relative/path" lines.
func writeBaseline(rootDir string, sums map[string]string) error {
	paths := make([]string, 0, len(sums))
	for path := range sums {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var content strings.Builder
	for _, path := range paths {
		content.WriteString(sums[path])
		content.WriteString("  ")
		content.WriteString(path)
		content.WriteString("\n")
	}
	target := filepath.Join(rootDir, baselineFile)
	if err := os.WriteFile(target, []byte(content.String()), 0o644); err != nil {
		return diagnostic("baseline_write_failed", "write upgrade baseline", target, err, "Check file permissions and available disk space.")
	}
	return nil
}

// writeBaselineFromDisk hashes every file under internal/platform as it sits
// on disk (post-gofmt) and writes the baseline. Used at scaffold time.
func writeBaselineFromDisk(rootDir string) error {
	sums := map[string]string{}
	platformDir := filepath.Join(rootDir, filepath.FromSlash(platformPrefix))
	err := filepath.WalkDir(platformDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		relFromRoot, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return relErr
		}
		rel := filepath.ToSlash(relFromRoot)
		sums[rel] = baselineHash(rel, raw)
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return diagnostic("baseline_write_failed", "hash platform files", platformDir, err, "Check directory permissions.")
	}
	return writeBaseline(rootDir, sums)
}
