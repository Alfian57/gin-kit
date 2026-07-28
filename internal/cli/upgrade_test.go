package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scaffoldStandaloneForUpgrade performs this package operation.
func scaffoldStandaloneForUpgrade(t *testing.T, mode string) (string, Manifest) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "app")
	m := Manifest{
		Version: 3, ProjectType: "standalone", Project: "app", Module: "example.com/app",
		Mode: mode, Database: "sqlite", ORM: "gorm",
	}
	if err := scaffold(dir, m); err != nil {
		t.Fatal(err)
	}
	return dir, m
}

// planByPath performs this package operation.
func planByPath(t *testing.T, rootDir string, m Manifest) map[string]upgradeEntry {
	t.Helper()
	entries, err := upgradePlan(rootDir, m)
	if err != nil {
		t.Fatal(err)
	}
	byPath := make(map[string]upgradeEntry, len(entries))
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}
	return byPath
}

// mutatePlatformFile performs this package operation.
func mutatePlatformFile(t *testing.T, rootDir, rel string) {
	t.Helper()
	path := filepath.Join(rootDir, filepath.FromSlash(rel))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(content, []byte("\n// local change\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStandaloneScaffoldWritesBaselineAndPlanIsClean(t *testing.T) {
	dir, m := scaffoldStandaloneForUpgrade(t, "api")
	baseline, err := readBaseline(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(baseline) == 0 {
		t.Fatal("scaffold did not write a .gin-kit.sum baseline")
	}
	for path := range baseline {
		if !strings.HasPrefix(path, platformPrefix) {
			t.Fatalf("baseline entry outside internal/platform: %s", path)
		}
	}
	entries, err := upgradePlan(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected platform files in the upgrade plan")
	}
	for _, entry := range entries {
		if entry.Status != statusUpToDate {
			t.Fatalf("fresh scaffold reports %s as %s", entry.Path, entry.Status)
		}
	}
}

func TestUpgradePlanClassifiesModifiedAndOutdated(t *testing.T) {
	dir, m := scaffoldStandaloneForUpgrade(t, "api")
	const rel = "internal/platform/query/query.go"

	// A local edit differs from both the render and the baseline.
	mutatePlatformFile(t, dir, rel)
	if got := planByPath(t, dir, m)[rel].Status; got != statusModified {
		t.Fatalf("locally edited file classified %s, want %s", got, statusModified)
	}

	// Pointing the baseline at the edited content simulates an old vendored
	// version: the file matches what the CLI once wrote, so it is outdated.
	edited, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := readBaseline(dir)
	if err != nil {
		t.Fatal(err)
	}
	baseline[rel] = baselineHash(rel, edited)
	if err := writeBaseline(dir, baseline); err != nil {
		t.Fatal(err)
	}
	if got := planByPath(t, dir, m)[rel].Status; got != statusOutdated {
		t.Fatalf("stale vendored file classified %s, want %s", got, statusOutdated)
	}
}

func TestUpgradePlanNormalizesGoBeforeComparingBaseline(t *testing.T) {
	dir, m := scaffoldStandaloneForUpgrade(t, "api")
	const rel = "internal/platform/query/query.go"
	path := filepath.Join(dir, filepath.FromSlash(rel))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Add a semantic change and deliberately make the file non-gofmt. Its
	// normalized checksum represents a prior generated version.
	edited := bytes.Replace(content, []byte("package query\n"), []byte("package query\n\n// prior generated version\n"), 1)
	edited = bytes.Replace(edited, []byte("import ("), []byte("import("), 1)
	if bytes.Equal(edited, normalizeGo(rel, edited)) {
		t.Fatal("test fixture unexpectedly remained gofmt-formatted")
	}
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	baseline, err := readBaseline(dir)
	if err != nil {
		t.Fatal(err)
	}
	baseline[rel] = baselineHash(rel, edited)
	if err := writeBaseline(dir, baseline); err != nil {
		t.Fatal(err)
	}

	if got := planByPath(t, dir, m)[rel].Status; got != statusOutdated {
		t.Fatalf("non-gofmt generated file classified %s, want %s", got, statusOutdated)
	}
}

func TestUpgradePlanWithoutBaseline(t *testing.T) {
	dir, m := scaffoldStandaloneForUpgrade(t, "api")
	if err := os.Remove(filepath.Join(dir, baselineFile)); err != nil {
		t.Fatal(err)
	}
	const mutated = "internal/platform/query/query.go"
	mutatePlatformFile(t, dir, mutated)
	byPath := planByPath(t, dir, m)
	if got := byPath[mutated].Status; got != statusDiffers {
		t.Fatalf("mutated file without baseline classified %s, want %s", got, statusDiffers)
	}
	const untouched = "internal/platform/httpx/response.go"
	if got := byPath[untouched].Status; got != statusUpToDate {
		t.Fatalf("untouched file without baseline classified %s, want %s", got, statusUpToDate)
	}
}

func TestApplyUpgradeWritesSafeUpdatesAndRefreshesBaseline(t *testing.T) {
	dir, m := scaffoldStandaloneForUpgrade(t, "api")
	const outdatedRel = "internal/platform/query/query.go"
	const modifiedRel = "internal/platform/httpx/bind.go"
	const missingRel = "internal/platform/config/config.go"
	const unmanagedRel = "internal/platform/response/response.go"

	// outdated: edited content recorded in the baseline (old vendored version).
	mutatePlatformFile(t, dir, outdatedRel)
	edited, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(outdatedRel)))
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := readBaseline(dir)
	if err != nil {
		t.Fatal(err)
	}
	baseline[outdatedRel] = baselineHash(outdatedRel, edited)
	if err := writeBaseline(dir, baseline); err != nil {
		t.Fatal(err)
	}
	// modified: edited content, baseline still holds the scaffold hash.
	mutatePlatformFile(t, dir, modifiedRel)
	// missing: rendered file deleted from disk.
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(missingRel))); err != nil {
		t.Fatal(err)
	}
	// unmanaged: a retired package the current templates do not render.
	unmanagedPath := filepath.Join(dir, filepath.FromSlash(unmanagedRel))
	if err := os.MkdirAll(filepath.Dir(unmanagedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	unmanagedContent := []byte("package response\n")
	if err := os.WriteFile(unmanagedPath, unmanagedContent, 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := upgradePlan(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	applied, skipped, err := applyUpgrade(dir, entries, false)
	if err != nil {
		t.Fatal(err)
	}
	appliedSet := map[string]bool{}
	for _, path := range applied {
		appliedSet[path] = true
	}
	if !appliedSet[outdatedRel] || !appliedSet[missingRel] {
		t.Fatalf("expected %s and %s applied, got %v", outdatedRel, missingRel, applied)
	}
	if len(skipped) != 1 || skipped[0] != modifiedRel {
		t.Fatalf("expected only %s skipped, got %v", modifiedRel, skipped)
	}

	// Applied files match the render again; the modified file kept its edit;
	// the unmanaged file was never touched.
	byPath := planByPath(t, dir, m)
	if got := byPath[outdatedRel].Status; got != statusUpToDate {
		t.Fatalf("%s after apply: %s, want %s", outdatedRel, got, statusUpToDate)
	}
	if got := byPath[missingRel].Status; got != statusUpToDate {
		t.Fatalf("%s after apply: %s, want %s", missingRel, got, statusUpToDate)
	}
	if got := byPath[modifiedRel].Status; got != statusModified {
		t.Fatalf("%s after apply without --force: %s, want %s", modifiedRel, got, statusModified)
	}
	if got := byPath[unmanagedRel].Status; got != statusUnmanaged {
		t.Fatalf("%s after apply: %s, want %s", unmanagedRel, got, statusUnmanaged)
	}
	afterUnmanaged, err := os.ReadFile(unmanagedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterUnmanaged) != string(unmanagedContent) {
		t.Fatal("apply touched an unmanaged file")
	}

	// The baseline now matches the restored files.
	baseline, err = readBaseline(dir)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(outdatedRel)))
	if err != nil {
		t.Fatal(err)
	}
	if baseline[outdatedRel] != baselineHash(outdatedRel, restored) {
		t.Fatal("baseline was not refreshed for the applied file")
	}
	if _, ok := baseline[unmanagedRel]; ok {
		t.Fatal("baseline gained an entry for an unmanaged file")
	}

	// --force overwrites the locally modified file.
	entries, err = upgradePlan(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	applied, skipped, err = applyUpgrade(dir, entries, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 1 || applied[0] != modifiedRel || len(skipped) != 0 {
		t.Fatalf("force apply: applied %v skipped %v", applied, skipped)
	}
	if got := planByPath(t, dir, m)[modifiedRel].Status; got != statusUpToDate {
		t.Fatalf("%s after force apply: %s, want %s", modifiedRel, got, statusUpToDate)
	}
}

func TestApplyUpgradeBootstrapsBaselineForOldProjects(t *testing.T) {
	dir, m := scaffoldStandaloneForUpgrade(t, "api")
	if err := os.Remove(filepath.Join(dir, baselineFile)); err != nil {
		t.Fatal(err)
	}
	entries, err := upgradePlan(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := applyUpgrade(dir, entries, false); err != nil {
		t.Fatal(err)
	}
	baseline, err := readBaseline(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(baseline) != len(entries) {
		t.Fatalf("bootstrap wrote %d baseline entries for %d matching files", len(baseline), len(entries))
	}
}

func TestUpgradePlanFollowsManifestGates(t *testing.T) {
	// An API-mode standalone renders no platform/session, so an on-disk session
	// file (for example from a UI project converted by hand) is unmanaged.
	dir, m := scaffoldStandaloneForUpgrade(t, "api")
	const sessionRel = "internal/platform/session/session.go"
	sessionPath := filepath.Join(dir, filepath.FromSlash(sessionRel))
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionPath, []byte("package session\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := planByPath(t, dir, m)[sessionRel].Status; got != statusUnmanaged {
		t.Fatalf("session file in api mode classified %s, want %s", got, statusUnmanaged)
	}
}

func TestUpgradePlanManagesSessionInUIMode(t *testing.T) {
	dir, m := scaffoldStandaloneForUpgrade(t, "ui")
	const sessionRel = "internal/platform/session/session.go"
	if got := planByPath(t, dir, m)[sessionRel].Status; got != statusUpToDate {
		t.Fatalf("session file in ui mode classified %s, want %s", got, statusUpToDate)
	}
}

func TestUpgradeCommandRejectsRuntimeProjectType(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fw")
	m := Manifest{
		Version: 3, ProjectType: "runtime", RuntimeVersion: "0.3.0",
		Project: "fw", Module: "example.com/fw",
		Mode: "api", Database: "sqlite", ORM: "gorm",
	}
	if err := scaffold(dir, m); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	cmd := upgradeCommand()
	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected the runtime project type to be rejected")
	}
	var diag *Diagnostic
	if !errors.As(err, &diag) || diag.Code != "upgrade_project_type_unsupported" {
		t.Fatalf("expected upgrade_project_type_unsupported diagnostic, got %v", err)
	}
	if !strings.Contains(diag.Recovery, "go get github.com/Alfian57/gin-kit@") {
		t.Fatalf("recovery does not point at the module upgrade path: %s", diag.Recovery)
	}
}
