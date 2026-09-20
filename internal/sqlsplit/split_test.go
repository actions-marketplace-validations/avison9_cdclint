package sqlsplit

import "testing"

func TestSplitTracksLinesAndHidesSemicolons(t *testing.T) {
	src := `-- a comment; with a semicolon
CREATE TABLE a (
  id UUID, -- trailing; comment
  note TEXT DEFAULT 'a;b'
);

/* block; comment
   spanning lines */
DO $$ BEGIN PERFORM 1; END $$;
ALTER TABLE a ADD COLUMN "x;y" INT;
`
	got := Split(src)
	want := []Statement{
		{Text: "CREATE TABLE a (\n  id UUID, \n  note TEXT DEFAULT 'a;b'\n)", Line: 2},
		{Text: "DO $$ BEGIN PERFORM 1; END $$", Line: 9},
		{Text: `ALTER TABLE a ADD COLUMN "x;y" INT`, Line: 10},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d statements, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Line != want[i].Line {
			t.Errorf("statement %d line = %d, want %d", i, got[i].Line, want[i].Line)
		}
		if got[i].Text != want[i].Text {
			t.Errorf("statement %d text = %q, want %q", i, got[i].Text, want[i].Text)
		}
	}
}
