package tmux

import (
	"strings"
	"testing"
)

// TestParsePanesOutputSurfacesFormatMismatchAsError guards against a real production bug: tmux
// doesn't escape embedded control bytes in -F format strings consistently across versions. An
// earlier version of ListPanesAll used "\x01" as a field separator on the assumption it could
// never collide with real content; on tmux 3.4 it instead escapes to the literal 4-character
// sequence "\001", which then never matches split("\x01") at all. Every line failed to parse,
// and the old code returned an empty map with a nil error — pixel-identical in the UI to "there
// really are zero sessions" (see: "md shows no sessions but tmux list-sessions has them").
// parsePanesOutput must detect "tmux printed real lines but none of them parsed" and return an
// error instead of silently looking like success.
func TestParsePanesOutputSurfacesFormatMismatchAsError(t *testing.T) {
	good := "$11\t3392473\t1794852817\t1\t1\t\t\t0 nvim api\tbash\tt\n" +
		"$9\t3392477\t1794852817\t1\t1\t\t\t0 nvim f\tbash\tt\n"

	// sanity check: the good input parses correctly first, proving the test fixture itself is valid
	res, err := parsePanesOutput(good)
	if err != nil {
		t.Fatalf("expected the well-formed fixture to parse cleanly, got error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 parsed sessions from the good fixture, got %d", len(res))
	}

	// reproduce the actual bug: every real tab replaced with the escaped literal sequence tmux
	// 3.4 produced for the old "\x01" separator — same shape of corruption, different byte
	mangled := strings.ReplaceAll(good, "\t", `\001`)

	_, err = parsePanesOutput(mangled)
	if err == nil {
		t.Fatal("expected a format-mismatch error, got nil — this is exactly the silent-empty-list bug")
	}
	if !strings.Contains(err.Error(), "didn't match the expected") {
		t.Fatalf("expected a descriptive format-mismatch error, got: %v", err)
	}
}

func TestParsePanesOutputHandlesNoSessions(t *testing.T) {
	res, err := parsePanesOutput("")
	if err != nil {
		t.Fatalf("expected no error for empty output, got %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected zero sessions, got %d", len(res))
	}
}

func TestParsePanesOutputPicksActivePaneForMultiPaneSession(t *testing.T) {
	// two rows for the same session_id ($5), only the second is the active pane
	out := "$5\t100\t1700000000\t1\t0\t\t\tmulti\tvim\ttitle-a\n" +
		"$5\t200\t1700000001\t1\t1\t\t\tmulti\tbash\ttitle-b\n"

	res, err := parsePanesOutput(out)
	if err != nil {
		t.Fatal(err)
	}
	info, ok := res["$5"]
	if !ok {
		t.Fatal("expected session $5 present")
	}
	if info.Command != "bash" || info.PID != 200 {
		t.Fatalf("expected the active pane (bash, pid 200) to win, got %+v", info)
	}
}

func TestParsePanesOutputTabInFreeTextFieldDoesNotDropTheRow(t *testing.T) {
	// a stray tab inside the LAST field (pane_title) must not cause the whole row to be
	// dropped — SplitN caps the split count so everything after the 9th real tab, including
	// any further tabs, is absorbed into the final field.
	out := "$7\t300\t1700000002\t1\t1\t\t\tname\tbash\ttitle\twith\tstray\ttabs\n"

	res, err := parsePanesOutput(out)
	if err != nil {
		t.Fatal(err)
	}
	info, ok := res["$7"]
	if !ok {
		t.Fatal("expected session $7 still parsed despite the stray tabs in its title field")
	}
	if info.SessionName != "name" || info.Command != "bash" {
		t.Fatalf("expected the structured fields before the title to be unaffected, got %+v", info)
	}
	if !strings.HasPrefix(info.Title, "title") {
		t.Fatalf("expected the title field to absorb the trailing content, got %q", info.Title)
	}
}
