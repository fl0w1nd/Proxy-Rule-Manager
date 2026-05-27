package transformer

import "testing"

func TestSampleLineDiff_DroppedOnly(t *testing.T) {
	dropped, droppedTotal, modified, modifiedTotal, added, addedTotal := SampleLineDiff(
		"# comment\nDOMAIN,a.com\nDOMAIN,b.com\n",
		"DOMAIN,a.com\nDOMAIN,b.com\n",
		"matched remove_lines pattern",
		"", // empty modifyReason → no pairing
		"", // empty addReason → no insert tracking
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
	if addedTotal != 0 || len(added) != 0 {
		t.Errorf("expected 0 added, got %d", addedTotal)
	}
}

func TestSampleLineDiff_ModifiedPair(t *testing.T) {
	dropped, droppedTotal, modified, modifiedTotal, added, addedTotal := SampleLineDiff(
		"DOMAIN,a.com\nMATCH,DIRECT\n",
		"DOMAIN,a.com\nFINAL,DIRECT\n",
		"regex removed line",
		"regex rewrote line",
		"regex inserted line",
	)
	if droppedTotal != 0 {
		t.Errorf("expected 0 drops, got %d (%v)", droppedTotal, dropped)
	}
	if addedTotal != 0 {
		t.Errorf("expected 0 added, got %d (%v)", addedTotal, added)
	}
	if modifiedTotal != 1 || len(modified) != 1 {
		t.Fatalf("expected 1 modified, got %d", modifiedTotal)
	}
	if modified[0].From != "MATCH,DIRECT" || modified[0].To != "FINAL,DIRECT" {
		t.Errorf("unexpected modified pair: %+v", modified[0])
	}
	if modified[0].LineNo != 2 {
		t.Errorf("unexpected lineNo: %d, want 2", modified[0].LineNo)
	}
}

func TestSampleLineDiff_NoChange(t *testing.T) {
	dropped, droppedTotal, modified, modifiedTotal, added, addedTotal := SampleLineDiff(
		"A\nB\nC\n",
		"A\nB\nC\n",
		"x", "y", "z",
	)
	if droppedTotal != 0 || modifiedTotal != 0 || addedTotal != 0 ||
		len(dropped) != 0 || len(modified) != 0 || len(added) != 0 {
		t.Errorf("unchanged content should produce no samples")
	}
}

func TestSampleLineDiff_RespectsCap(t *testing.T) {
	const extra = 10
	want := MaxReportSamples + extra
	var before, after string
	for i := 0; i < want; i++ {
		before += "X\n"
	}
	dropped, droppedTotal, _, _, _, _ := SampleLineDiff(before, after, "drop", "", "")
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
	dropped, _, _, _, _, _ := SampleLineDiff(before, "", "drop", "", "")
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

// TestSampleLineDiff_PureDropsNotMisreportedAsModified reproduces the
// "DOMAIN,aws-na.intlgame.com → DOMAIN,www.jupiterlauncher.com" failure
// from the dashboard transformer pipeline panel. The user's redundant_cleaner
// transformer only ever drops lines; the previous heuristic paired each
// vanished line with the next unmatched output line, producing nonsensical
// modify pairs. The opcode-based implementation must classify these as drops.
func TestSampleLineDiff_PureDropsNotMisreportedAsModified(t *testing.T) {
	before := "DOMAIN,aws-na.intlgame.com\n" +
		"DOMAIN,www.jupiterlauncher.com\n" +
		"DOMAIN,global-lobby.nikke-kr.com\n" +
		"IP-CIDR,43.152.114.31/32,no-resolve\n" +
		"DOMAIN,cloud.nikke-kr.com\n" +
		"IP-CIDR,43.130.245.250/24,no-resolve\n" +
		"DOMAIN,sg-vas.intlgame.com\n" +
		"DOMAIN-SUFFIX,nikke-kr.com\n"
	after := "DOMAIN,www.jupiterlauncher.com\n" +
		"IP-CIDR,43.152.114.31/32,no-resolve\n" +
		"IP-CIDR,43.130.245.250/24,no-resolve\n" +
		"DOMAIN-SUFFIX,nikke-kr.com\n"
	dropped, droppedTotal, modified, modifiedTotal, added, addedTotal := SampleLineDiff(
		before, after,
		"redundant_cleaner",
		"redundant_cleaner",
		"redundant_cleaner",
	)
	if modifiedTotal != 0 || len(modified) != 0 {
		t.Errorf("expected 0 modified pairs, got %d (%+v)", modifiedTotal, modified)
	}
	if addedTotal != 0 || len(added) != 0 {
		t.Errorf("expected 0 added lines, got %d (%+v)", addedTotal, added)
	}
	if droppedTotal != 4 || len(dropped) != 4 {
		t.Fatalf("expected 4 dropped lines, got %d (%+v)", droppedTotal, dropped)
	}
	wantDrops := []string{
		"DOMAIN,aws-na.intlgame.com",
		"DOMAIN,global-lobby.nikke-kr.com",
		"DOMAIN,cloud.nikke-kr.com",
		"DOMAIN,sg-vas.intlgame.com",
	}
	for i, want := range wantDrops {
		if dropped[i].Text != want {
			t.Errorf("dropped[%d].Text = %q, want %q", i, dropped[i].Text, want)
		}
	}
}

// TestSampleLineDiff_InsertedTracked verifies a 'pure insert' opcode lands
// in the Added bucket (not the Modified bucket) when addReason is set.
func TestSampleLineDiff_InsertedTracked(t *testing.T) {
	before := "DOMAIN,a.com\nDOMAIN,c.com\n"
	after := "DOMAIN,a.com\nDOMAIN,b.com\nDOMAIN,c.com\n"
	dropped, _, modified, _, added, addedTotal := SampleLineDiff(
		before, after, "drop", "modify", "insert",
	)
	if len(dropped) != 0 || len(modified) != 0 {
		t.Errorf("expected pure insert, got drops=%v modified=%v", dropped, modified)
	}
	if addedTotal != 1 || len(added) != 1 {
		t.Fatalf("expected 1 inserted, got %d (%+v)", addedTotal, added)
	}
	if added[0].Text != "DOMAIN,b.com" {
		t.Errorf("unexpected inserted text: %q", added[0].Text)
	}
	if added[0].LineNo != 2 {
		t.Errorf("unexpected lineNo: %d, want 2", added[0].LineNo)
	}
}

// TestSampleLineDiff_AddReasonEmptyDisablesAddTrack confirms that callers
// who opt out of the added track (e.g. remove_lines) get a zero count even
// when an opcode-level insert occurs.
func TestSampleLineDiff_AddReasonEmptyDisablesAddTrack(t *testing.T) {
	before := "A\nB\n"
	after := "A\nB\nC\n"
	dropped, _, _, _, added, addedTotal := SampleLineDiff(
		before, after, "drop", "", "",
	)
	if len(dropped) != 0 {
		t.Errorf("did not expect drops, got %v", dropped)
	}
	if addedTotal != 0 || len(added) != 0 {
		t.Errorf("expected added track disabled, got %d (%v)", addedTotal, added)
	}
}

// TestSampleLineDiff_AsymmetricReplaceSplitsIntoDropAndAdd verifies that
// a replace opcode with unequal lengths never invents a partial pairing —
// it spills into Dropped + Added so the UI shows the real semantics.
func TestSampleLineDiff_AsymmetricReplaceSplitsIntoDropAndAdd(t *testing.T) {
	before := "X\nY\n"
	after := "P\nQ\nR\n"
	dropped, droppedTotal, modified, modifiedTotal, added, addedTotal := SampleLineDiff(
		before, after, "drop", "modify", "insert",
	)
	if modifiedTotal != 0 || len(modified) != 0 {
		t.Errorf("expected no modified pairs, got %d (%+v)", modifiedTotal, modified)
	}
	if droppedTotal != 2 || len(dropped) != 2 {
		t.Errorf("expected 2 drops, got %d", droppedTotal)
	}
	if addedTotal != 3 || len(added) != 3 {
		t.Errorf("expected 3 adds, got %d", addedTotal)
	}
}
