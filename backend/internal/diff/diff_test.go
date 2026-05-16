package diff

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCreateActivityDiff_LargeCreated(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 600; i++ {
		sb.WriteString("DOMAIN,line-")
		sb.WriteString(itoa(i))
		sb.WriteString(".example\n")
	}
	out := CreateActivityDiff(Created, "", sb.String(), 3)
	for _, want := range []string{
		"# created summary",
		"# full diff omitted for large payload",
		"# lines: 601", // trailing newline pushes count past 600 in Go's counting
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestCreateActivityDiff_UpdatedKeepsFullDiff(t *testing.T) {
	out := CreateActivityDiff(Updated, "DOMAIN,before.example", "DOMAIN,after.example", 3)
	if !strings.Contains(out, "--- before") {
		t.Errorf("missing --- before header: %s", out)
	}
	if !strings.Contains(out, "+++ after") {
		t.Errorf("missing +++ after header: %s", out)
	}
}

func TestCreateActivityDiff_LargeDeleted(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 700; i++ {
		sb.WriteString("DOMAIN,deleted-")
		sb.WriteString(itoa(i))
		sb.WriteString(".example\n")
	}
	out := CreateActivityDiff(Deleted, sb.String(), "", 3)
	if !strings.Contains(out, "# deleted summary") {
		t.Errorf("missing deleted summary in %s", out)
	}
	if !strings.Contains(out, "# lines: 701") {
		t.Errorf("missing line count in %s", out)
	}
}

func TestCreateActivityDiff_SmallCreatedKeepsFullDiff(t *testing.T) {
	out := CreateActivityDiff(Created, "", "DOMAIN,one.example", 3)
	if strings.Contains(out, "summary") {
		t.Errorf("small payload should not be summarized: %s", out)
	}
	if !strings.Contains(out, "+++ after") {
		t.Errorf("expected unified diff, got: %s", out)
	}
}

func TestCreateLineDiff_DefaultContext(t *testing.T) {
	out := CreateLineDiff("a\nb\nc\n", "a\nB\nc\n", 0)
	if !strings.Contains(out, "-b") {
		t.Errorf("expected removal line, got: %s", out)
	}
	if !strings.Contains(out, "+B") {
		t.Errorf("expected addition line, got: %s", out)
	}
}

// TestCreateLineDiff_IndexHeader verifies that the output includes the jsdiff-
// compatible "Index: before\n===…\n" prefix expected by the frontend diff-viewer.
func TestCreateLineDiff_IndexHeader(t *testing.T) {
	out := CreateLineDiff("a\n", "b\n", 3)
	if !strings.HasPrefix(out, "Index: before\n") {
		t.Errorf("expected Index: before header, got: %s", out)
	}
	if !strings.Contains(out, "===") {
		t.Errorf("expected separator line, got: %s", out)
	}
}

// TestShouldSummarize_ASCIIByteTrigger verifies that 50 001 ASCII bytes (where
// byte count == rune count) triggers the summary path.
func TestShouldSummarize_ASCIIByteTrigger(t *testing.T) {
	// Build content that is just over 50 000 ASCII characters.
	content := strings.Repeat("a", 50001)
	if utf8.RuneCountInString(content) <= 50000 {
		t.Fatalf("precondition: expected > 50000 runes")
	}
	out := CreateActivityDiff(Created, "", content, 3)
	if !strings.Contains(out, "summary") {
		t.Errorf("expected summary for 50001 ASCII chars, got: %s", out)
	}
}

// TestShouldSummarize_CJKRuneTrigger verifies that content whose byte length is
// far above 50 000 (due to multi-byte CJK runes) but whose rune count is also
// above 50 000 correctly triggers the summary path. This guards against
// accidentally using len() (bytes) instead of RuneCountInString().
func TestShouldSummarize_CJKRuneTrigger(t *testing.T) {
	// Each CJK character is 3 UTF-8 bytes; 50 001 of them = 150 003 bytes.
	content := strings.Repeat("中", 50001)
	if utf8.RuneCountInString(content) <= 50000 {
		t.Fatalf("precondition: expected > 50000 runes")
	}
	out := CreateActivityDiff(Created, "", content, 3)
	if !strings.Contains(out, "summary") {
		t.Errorf("expected summary for 50001 CJK runes, got: %s", out)
	}
}

// TestShouldSummarize_CJKUnderRuneThreshold verifies that content whose byte
// count is above 50 000 but whose rune count is below the threshold is NOT
// summarized. Without the rune-count fix, len() on 17 000 CJK chars would give
// 51 000 bytes and incorrectly trigger the summary.
func TestShouldSummarize_CJKUnderRuneThreshold(t *testing.T) {
	// 10 000 CJK chars × 3 bytes = 30 000 bytes; 10 000 runes < 50 000.
	content := strings.Repeat("中", 10000)
	if utf8.RuneCountInString(content) >= 50000 {
		t.Fatalf("precondition: expected < 50000 runes")
	}
	// With fewer than 500 lines and fewer than 50 000 runes, no summary.
	out := CreateActivityDiff(Created, "", content, 3)
	if strings.Contains(out, "summary") {
		t.Errorf("unexpected summary for 10000 CJK runes (< threshold), got: %s", out)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
