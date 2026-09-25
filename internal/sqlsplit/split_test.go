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

func TestSplitMySQLHidesItsOwnSemicolons(t *testing.T) {
	src := "# a hash comment; with a semicolon\n" +
		"CREATE TABLE `a` (\n" +
		"  id INT, # trailing; comment\n" +
		"  note VARCHAR(20) DEFAULT 'it\\'s; fine'\n" +
		") ENGINE=InnoDB;\n" +
		"/*!40101 SET NAMES utf8mb4 */;\n" +
		"DELIMITER $$\n" +
		"CREATE TRIGGER a_bi BEFORE INSERT ON a FOR EACH ROW BEGIN SET NEW.id = 1; END$$\n" +
		"DELIMITER ;\n" +
		"ALTER TABLE a ADD COLUMN price$ INT;\n"
	got := SplitMySQL(src)
	want := []Statement{
		{Text: "CREATE TABLE `a` (\n  id INT, \n  note VARCHAR(20) DEFAULT 'it\\'s; fine'\n) ENGINE=InnoDB", Line: 2},
		{Text: "CREATE TRIGGER a_bi BEFORE INSERT ON a FOR EACH ROW BEGIN SET NEW.id = 1; END", Line: 8},
		{Text: "ALTER TABLE a ADD COLUMN price$ INT", Line: 10},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d statements, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("statement %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}
