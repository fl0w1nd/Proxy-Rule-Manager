package transformer

import "testing"

func TestSampleLineDiff_DroppedOnly(t *testing.T) {
	dropped, droppedTotal, modified, modifiedTotal := SampleLineDiff(
		"# comment\nDOMAIN,a.com\nDOMAIN,b.com\n",
		"DOMAIN,a.com\nDOMAIN,b.com\n",
		"matched remove_lines pattern",
		"", // empty modifyReason → no pairing
	)
	if droppedTotal != 1 || len(dropped) != 1 {
		t.Fatalf("expected 1 drop, got %d (samples=%v)", droppedTotal, dropped)
	}
	if dropped[0].Text != "# comment" {
		t.Errorf("unexpected dropped text: %q", dropped[0].Text)
	}
	if modifiedTotal != 0 || len(modified) != 0 {
		t.Errorf("expected 0 modified, got %d", modifiedTotal)
	}
}

func TestSampleLineDiff_ModifiedPair(t *testing.T) {
	dropped, droppedTotal, modified, modifiedTotal := SampleLineDiff(
		"DOMAIN,a.com\nMATCH,DIRECT\n",
		"DOMAIN,a.com\nFINAL,DIRECT\n",
		"regex removed line",
		"regex rewrote line",
	)
	if droppedTotal != 0 {
		t.Errorf("expected 0 drops, got %d (%v)", droppedTotal, dropped)
	}
	if modifiedTotal != 1 || len(modified) != 1 {
		t.Fatalf("expected 1 modified, got %d", modifiedTotal)
	}
	if modified[0].From != "MATCH,DIRECT" || modified[0].To != "FINAL,DIRECT" {
		t.Errorf("unexpected modified pair: %+v", modified[0])
	}
}

func TestSampleLineDiff_NoChange(t *testing.T) {
	dropped, droppedTotal, modified, modifiedTotal := SampleLineDiff(
		"A\nB\nC\n",
		"A\nB\nC\n",
		"x", "y",
	)
	if droppedTotal != 0 || modifiedTotal != 0 || len(dropped) != 0 || len(modified) != 0 {
		t.Errorf("unchanged content should produce no samples")
	}
}

func TestSampleLineDiff_RespectsCap(t *testing.T) {
	// Generate a long input where MaxReportSamples+10 lines are dropped.
	const extra = 10
	want := MaxReportSamples + extra
	var before, after string
	for i := 0; i < want; i++ {
		before += "X\n"
	}
	dropped, droppedTotal, _, _ := SampleLineDiff(before, after, "drop", "")
	if droppedTotal != want {
		t.Errorf("droppedTotal = %d, want %d", droppedTotal, want)
	}
	if len(dropped) != MaxReportSamples {
		t.Errorf("samples len = %d, want %d", len(dropped), MaxReportSamples)
	}
}

func TestSampleLineDiff_LongLineTruncated(t *testing.T) {
	long := ""
	for i := 0; i < MaxSampleBytes+200; i++ {
		long += "x"
	}
	before := long + "\n"
	dropped, _, _, _ := SampleLineDiff(before, "", "drop", "")
	if len(dropped) != 1 {
		t.Fatalf("expected 1 dropped sample, got %d", len(dropped))
	}
	if !dropped[0].Truncated {
		t.Errorf("expected Truncated=true")
	}
	if len(dropped[0].Text) > MaxSampleBytes {
		t.Errorf("sample text exceeds cap: %d", len(dropped[0].Text))
	}
}
