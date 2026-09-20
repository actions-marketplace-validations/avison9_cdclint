package debezium

import "testing"

func TestIncludeListsAndTopics(t *testing.T) {
	c, err := Parse([]byte(`{"name":"pg","config":{
		"connector.class":"io.debezium.connector.postgresql.PostgresConnector",
		"topic.prefix":"rr",
		"table.include.list":"public.reports,public.users",
		"column.include.list":"public.reports\\.(id|category),public.users.id",
		"transforms":"route",
		"transforms.route.type":"org.apache.kafka.connect.transforms.RegexRouter",
		"transforms.route.regex":"rr\\.public\\.(.*)",
		"transforms.route.replacement":"cdc.$1"
	}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !c.CapturesTable("public", "reports") || c.CapturesTable("public", "media") {
		t.Error("table include list misjudged")
	}
	if !c.CapturesColumn("public", "reports", "id") || !c.CapturesColumn("public", "reports", "CATEGORY") {
		t.Error("column include list misjudged an included column")
	}
	if c.CapturesColumn("public", "reports", "reference_number") {
		t.Error("column include list matched a column it does not list")
	}
	if got := c.Topic("public", "reports"); got != "cdc.reports" {
		t.Errorf("topic = %q, want cdc.reports", got)
	}
	if c.ColumnListSetting() != "column.include.list" {
		t.Errorf("column list key = %q", c.ColumnListSetting())
	}
}

func TestNoListsCapturesEverything(t *testing.T) {
	c, err := Parse([]byte(`{"connector.class":"x","topic.prefix":"p"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !c.CapturesTable("a", "b") || !c.CapturesColumn("a", "b", "c") {
		t.Error("empty lists should capture everything")
	}
	if c.Topic("a", "b") != "p.a.b" {
		t.Errorf("topic = %q", c.Topic("a", "b"))
	}
}

func TestExcludeList(t *testing.T) {
	c, _ := Parse([]byte(`{"column.exclude.list":"public.users.password_hash"}`))
	if c.CapturesColumn("public", "users", "password_hash") || !c.CapturesColumn("public", "users", "id") {
		t.Error("exclude list misjudged")
	}
	if c.ColumnListSetting() != "column.exclude.list" {
		t.Errorf("column list key = %q", c.ColumnListSetting())
	}
}
