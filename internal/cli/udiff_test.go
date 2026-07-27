package cli

import (
	"strings"
	"testing"
)

func TestUnifiedDiffEqualInputsAreEmpty(t *testing.T) {
	content := []byte("alpha\nbeta\n")
	if got := unifiedDiff("file.go", content, content); got != "" {
		t.Fatalf("equal inputs produced a diff:\n%s", got)
	}
}

func TestUnifiedDiffInsert(t *testing.T) {
	a := []byte("one\ntwo\nthree\n")
	b := []byte("one\ntwo\nnew line\nthree\n")
	got := unifiedDiff("pkg/file.go", a, b)
	want := "--- a/pkg/file.go\n" +
		"+++ b/pkg/file.go\n" +
		"@@ -1,3 +1,4 @@\n" +
		" one\n" +
		" two\n" +
		"+new line\n" +
		" three\n"
	if got != want {
		t.Fatalf("insert diff mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestUnifiedDiffDelete(t *testing.T) {
	a := []byte("one\ntwo\nthree\n")
	b := []byte("one\nthree\n")
	got := unifiedDiff("file.txt", a, b)
	want := "--- a/file.txt\n" +
		"+++ b/file.txt\n" +
		"@@ -1,3 +1,2 @@\n" +
		" one\n" +
		"-two\n" +
		" three\n"
	if got != want {
		t.Fatalf("delete diff mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestUnifiedDiffReplaceWithContext(t *testing.T) {
	a := []byte("l1\nl2\nl3\nl4\nl5\nold\nl7\nl8\nl9\nl10\n")
	b := []byte("l1\nl2\nl3\nl4\nl5\nnew\nl7\nl8\nl9\nl10\n")
	got := unifiedDiff("x", a, b)
	want := "--- a/x\n" +
		"+++ b/x\n" +
		"@@ -3,7 +3,7 @@\n" +
		" l3\n" +
		" l4\n" +
		" l5\n" +
		"-old\n" +
		"+new\n" +
		" l7\n" +
		" l8\n" +
		" l9\n"
	if got != want {
		t.Fatalf("replace diff mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestUnifiedDiffSeparatesDistantHunks(t *testing.T) {
	var aLines, bLines []string
	for i := 1; i <= 20; i++ {
		line := "line" + string(rune('a'+i-1))
		aLines = append(aLines, line)
		bLines = append(bLines, line)
	}
	bLines[0] = "changed-top"
	bLines[19] = "changed-bottom"
	a := []byte(strings.Join(aLines, "\n") + "\n")
	b := []byte(strings.Join(bLines, "\n") + "\n")
	got := unifiedDiff("x", a, b)
	if strings.Count(got, "@@") != 4 { // two hunks, two markers each
		t.Fatalf("expected two hunks:\n%s", got)
	}
	if !strings.Contains(got, "@@ -1,4 +1,4 @@") || !strings.Contains(got, "@@ -17,4 +17,4 @@") {
		t.Fatalf("hunk headers wrong:\n%s", got)
	}
	if !strings.Contains(got, "+changed-top\n") || !strings.Contains(got, "+changed-bottom\n") {
		t.Fatalf("hunk bodies wrong:\n%s", got)
	}
}

func TestUnifiedDiffMergesNearbyChanges(t *testing.T) {
	a := []byte("l1\nl2\nold-a\nl4\nl5\nl6\nold-b\nl8\nl9\n")
	b := []byte("l1\nl2\nnew-a\nl4\nl5\nl6\nnew-b\nl8\nl9\n")
	got := unifiedDiff("x", a, b)
	if strings.Count(got, "@@") != 2 { // changes 3 lines apart share one hunk
		t.Fatalf("expected a single merged hunk:\n%s", got)
	}
	if !strings.Contains(got, "@@ -1,9 +1,9 @@") {
		t.Fatalf("merged hunk header wrong:\n%s", got)
	}
}

func TestUnifiedDiffFromEmpty(t *testing.T) {
	got := unifiedDiff("new.go", nil, []byte("package x\n"))
	want := "--- a/new.go\n" +
		"+++ b/new.go\n" +
		"@@ -0,0 +1,1 @@\n" +
		"+package x\n"
	if got != want {
		t.Fatalf("creation diff mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestUnifiedDiffMissingTrailingNewline(t *testing.T) {
	got := unifiedDiff("x", []byte("same\nlast"), []byte("same\nlast\n"))
	if !strings.Contains(got, "-last\n\\ No newline at end of file\n") || !strings.Contains(got, "+last\n") {
		t.Fatalf("missing-newline diff mismatch:\n%s", got)
	}
}
