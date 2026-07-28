package cli

import (
	"bytes"
	"fmt"
	"strings"
)

// unifiedDiff renders a minimal line-based unified diff between a and b with
// three lines of context, labeled a/<name> and b/<name>. Equal inputs return
// the empty string. It exists so gin-kit upgrade --diff needs no dependency.
func unifiedDiff(name string, a, b []byte) string {
	if bytes.Equal(a, b) {
		return ""
	}
	ops := diffOps(splitDiffLines(a), splitDiffLines(b))

	const context = 3
	var out strings.Builder
	fmt.Fprintf(&out, "--- a/%s\n+++ b/%s\n", name, name)
	for i := 0; i < len(ops); {
		if ops[i].kind == ' ' {
			i++
			continue
		}
		// Start the hunk up to `context` lines before the first change, then
		// extend it while further changes sit within 2*context equal lines.
		start := i - context
		if start < 0 {
			start = 0
		}
		lastChange := i
		j := i + 1
		for j < len(ops) {
			if ops[j].kind != ' ' {
				lastChange = j
				j++
				continue
			}
			gap := j
			for gap < len(ops) && ops[gap].kind == ' ' {
				gap++
			}
			if gap == len(ops) || gap-j > 2*context {
				break
			}
			j = gap
		}
		end := lastChange + context
		if end > len(ops)-1 {
			end = len(ops) - 1
		}
		writeHunk(&out, ops[start:end+1])
		i = end + 1
	}
	return out.String()
}

// diffOp is one line of a computed diff: kept (' '), deleted ('-'), or
// inserted ('+'). Lines retain their trailing newline when present so a
// missing final newline still diffs.
type diffOp struct {
	// kind store data used by this type.
	kind byte
	// text store data used by this type.
	text  string
	aLine int // 1-based position in a (next a line for '+' ops)
	bLine int // 1-based position in b (next b line for '-' ops)
}

// diffOps aligns a and b on their longest common subsequence of lines.
func diffOps(a, b []string) []diffOp {
	// lcs[i][j] is the LCS length of a[i:] and b[j:].
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	var ops []diffOp
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			ops = append(ops, diffOp{' ', a[i], i + 1, j + 1})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, diffOp{'-', a[i], i + 1, j + 1})
			i++
		default:
			ops = append(ops, diffOp{'+', b[j], i + 1, j + 1})
			j++
		}
	}
	for ; i < len(a); i++ {
		ops = append(ops, diffOp{'-', a[i], i + 1, j + 1})
	}
	for ; j < len(b); j++ {
		ops = append(ops, diffOp{'+', b[j], i + 1, j + 1})
	}
	return ops
}

// writeHunk performs this package operation.
func writeHunk(out *strings.Builder, ops []diffOp) {
	aCount, bCount := 0, 0
	for _, op := range ops {
		if op.kind != '+' {
			aCount++
		}
		if op.kind != '-' {
			bCount++
		}
	}
	// An empty side positions the hunk after the preceding line, per the
	// unified format (count 0 means "insert here", start may be 0).
	aStart, bStart := ops[0].aLine, ops[0].bLine
	if aCount == 0 {
		aStart--
	}
	if bCount == 0 {
		bStart--
	}
	fmt.Fprintf(out, "@@ -%d,%d +%d,%d @@\n", aStart, aCount, bStart, bCount)
	for _, op := range ops {
		out.WriteByte(op.kind)
		out.WriteString(strings.TrimSuffix(op.text, "\n"))
		out.WriteByte('\n')
		if !strings.HasSuffix(op.text, "\n") {
			out.WriteString("\\ No newline at end of file\n")
		}
	}
}

// splitDiffLines splits content into lines that keep their "\n" terminator,
// so "x" and "x\n" compare as different final lines.
func splitDiffLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	lines := strings.SplitAfter(string(data), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
