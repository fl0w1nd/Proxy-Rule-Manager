package ir

import (
	"strings"
	"testing"
)

func TestLintClassicalReportsLinesAndDiagnostics(t *testing.T) {
	content := "# comment\nDOMAIN-SUFFIX,example.com\n\nexample.org\nFOO,bar\nIP-CIDR,10.0.0.0/8,no-resolve\nMATCH,DIRECT\n"
	report := Lint(content, 10)
	if report.Format != FormatClassical || report.Entries != 3 || report.DiagnosticCount != 2 {
		t.Fatalf("report=%+v", report)
	}
	if got := report.Diagnostics[0]; got.Line != 5 || got.Text != "FOO,bar" || !strings.Contains(got.Reason, "unknown rule type") {
		t.Fatalf("diagnostic=%+v", got)
	}
	if report.Diagnostics[1].Line != 7 {
		t.Fatalf("diagnostic=%+v", report.Diagnostics[1])
	}
	want := []LintLine{
		{Line: 2, Text: "DOMAIN-SUFFIX,example.com", Kind: KindDomainSuffix, Value: "example.com"},
		{Line: 4, Text: "example.org", Kind: KindDomain, Value: "example.org"},
		{Line: 6, Text: "IP-CIDR,10.0.0.0/8,no-resolve", Kind: KindIPCIDR, Value: "10.0.0.0/8,no-resolve"},
	}
	if len(report.Lines) != len(want) {
		t.Fatalf("lines=%+v", report.Lines)
	}
	for i := range want {
		if report.Lines[i] != want[i] {
			t.Fatalf("line %d = %+v, want %+v", i, report.Lines[i], want[i])
		}
	}
	if len(report.Kinds) != 3 {
		t.Fatalf("kinds=%+v", report.Kinds)
	}
}

func TestLintMatchesParse(t *testing.T) {
	for _, content := range []string{
		"DOMAIN,a.com\nbad line here\n1.2.3.4/33\n",
		"payload:\n  - DOMAIN,a.com\n  - '+.b.com'\n  - 'NOPE,x'\n",
		"payload:\n",
	} {
		rs, det, err := Parse(content)
		report := Lint(content, 0)
		if err != nil {
			if report.DiagnosticCount != 1 || report.Diagnostics[0].Reason != ErrUndetectedFormat {
				t.Fatalf("%q: report=%+v", content, report)
			}
			continue
		}
		if report.Format != det.Format || report.Entries != len(rs.Entries) || report.DiagnosticCount != len(rs.Diagnostics) {
			t.Fatalf("%q: report=%+v parse=%+v", content, report, rs)
		}
		for i, d := range rs.Diagnostics {
			if report.Diagnostics[i] != d {
				t.Fatalf("%q: diagnostic %d = %+v, want %+v", content, i, report.Diagnostics[i], d)
			}
		}
		if len(report.Lines) != 0 || report.LinesTruncated {
			t.Fatalf("lines should be disabled: %+v", report.Lines)
		}
	}
}

func TestLintYAMLLineNumbers(t *testing.T) {
	report := Lint("payload:\n  - DOMAIN,a.com\n  - 'NOPE,x'\n", 5)
	if report.Format != FormatMihomoYAML || report.Entries != 1 || report.DiagnosticCount != 1 {
		t.Fatalf("report=%+v", report)
	}
	if report.Lines[0].Line != 2 || report.Diagnostics[0].Line != 3 {
		t.Fatalf("lines=%+v diagnostics=%+v", report.Lines, report.Diagnostics)
	}
}

func TestLintUndetectedAndTruncated(t *testing.T) {
	for _, content := range []string{"", "# only comments\n"} {
		report := Lint(content, 5)
		if report.Format != "" || report.DiagnosticCount != 1 || report.Diagnostics[0].Line != 0 {
			t.Fatalf("%q: report=%+v", content, report)
		}
	}
	report := Lint("a.com\nb.com\nc.com\n", 2)
	if len(report.Lines) != 2 || !report.LinesTruncated || report.Entries != 3 {
		t.Fatalf("report=%+v", report)
	}
	var b strings.Builder
	b.WriteString("DOMAIN,ok.com\n")
	for range MaxLintDiagnostics + 10 {
		b.WriteString("NOPE,x\n")
	}
	report = Lint(b.String(), 0)
	if len(report.Diagnostics) != MaxLintDiagnostics || report.DiagnosticCount != MaxLintDiagnostics+10 {
		t.Fatalf("diagnostics=%d count=%d", len(report.Diagnostics), report.DiagnosticCount)
	}
}

func TestRuleTypeSpellingsCoverEveryKind(t *testing.T) {
	spellings := RuleTypeSpellings()
	canonical := map[Kind]string{}
	seenAlias := false
	for _, s := range spellings {
		if s.Alias {
			seenAlias = true
			continue
		}
		if seenAlias {
			t.Fatalf("canonical %s listed after aliases", s.Name)
		}
		if _, dup := canonical[s.Kind]; dup {
			t.Fatalf("duplicate canonical spelling for %s", s.Kind)
		}
		canonical[s.Kind] = s.Name
	}
	for _, kind := range AllKinds {
		if canonical[kind] == "" {
			t.Fatalf("kind %s has no canonical spelling", kind)
		}
	}
	if spellings[0].Name != "DOMAIN" {
		t.Fatalf("first spelling = %s", spellings[0].Name)
	}
	if flags := RuleLineFlags(); len(flags) == 0 || flags[0] > flags[len(flags)-1] {
		t.Fatalf("flags=%v", flags)
	}
}
