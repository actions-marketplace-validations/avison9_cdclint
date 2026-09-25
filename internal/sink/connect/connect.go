// Package connect reads Kafka Connect SINK connector configs and answers
// which topic a warehouse table is fed from. Each sink connector has its own
// mapping setting and its own rule for the table name when the setting is
// absent; this package knows the four that ship in v1 and treats anything
// else as "table name equals topic name".
package connect

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/avison9/cdclint/internal/model"
	"github.com/avison9/cdclint/internal/smt"
)

// Kind is which sink connector family a config belongs to.
type Kind string

const (
	Snowflake  Kind = "snowflake"
	BigQuery   Kind = "bigquery"
	Iceberg    Kind = "iceberg"
	ClickHouse Kind = "clickhouse"
	Unknown    Kind = "unknown"
)

// Mapper is one sink connector's topic-to-table knowledge.
type Mapper struct {
	Kind        Kind
	Pos         model.Pos
	Class       string
	topics      []string
	topicsRegex *regexp.Regexp
	explicit    map[string]string // topic -> table, from the connector's map setting
	setting     string            // which setting held the map
	routes      []route           // Iceberg: table -> route regex
	single      string            // Iceberg: the one table when iceberg.tables lists one
	cfg         map[string]string // the whole config, for its transforms
}

type route struct {
	table string
	re    *regexp.Regexp
}

// Reshape applies this sink connector's transforms to the value the source
// connector produced; a sink connector can unwrap or flatten on its side.
func (m *Mapper) Reshape(in model.Shape) model.Shape {
	return smt.Apply(m.cfg, m.Pos.File, in)
}

// ReadFile parses one sink connector JSON file.
func ReadFile(path string) (*Mapper, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m, err := Parse(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	m.Pos = model.Pos{File: path}
	return m, nil
}

// Parse builds a Mapper from JSON, full document or bare config.
func Parse(b []byte) (*Mapper, error) {
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	cfg := map[string]string{}
	src := doc
	if inner, ok := doc["config"].(map[string]any); ok {
		src = inner
	}
	for k, v := range src {
		cfg[k] = fmt.Sprint(v)
	}
	m := &Mapper{Class: cfg["connector.class"], explicit: map[string]string{}, cfg: cfg}
	lc := strings.ToLower(m.Class)
	switch {
	case strings.Contains(lc, "snowflake"):
		m.Kind = Snowflake
		m.setting = "snowflake.topic2table.map"
		m.parsePairs(cfg[m.setting], ":")
	case strings.Contains(lc, "bigquery"):
		m.Kind = BigQuery
		m.setting = "topic2TableMap"
		m.parsePairs(cfg[m.setting], ":")
	case strings.Contains(lc, "iceberg"):
		m.Kind = Iceberg
		m.setting = "iceberg.tables"
		var tables []string
		for _, t := range strings.Split(cfg["iceberg.tables"], ",") {
			if t = strings.TrimSpace(t); t != "" {
				tables = append(tables, t)
			}
		}
		if len(tables) == 1 {
			m.single = tables[0]
		}
		for _, t := range tables {
			if re := cfg["iceberg.table."+t+".route-regex"]; re != "" {
				if c, err := regexp.Compile("^(?:" + re + ")$"); err == nil {
					m.routes = append(m.routes, route{table: t, re: c})
				}
			}
		}
	case strings.Contains(lc, "clickhouse"):
		m.Kind = ClickHouse
		m.setting = "topic2TableMap"
		m.parsePairs(cfg[m.setting], "=")
	default:
		m.Kind = Unknown
	}
	for _, t := range strings.Split(cfg["topics"], ",") {
		if t = strings.TrimSpace(t); t != "" {
			m.topics = append(m.topics, t)
		}
	}
	if re := cfg["topics.regex"]; re != "" {
		if c, err := regexp.Compile("^(?:" + re + ")$"); err == nil {
			m.topicsRegex = c
		}
	}
	return m, nil
}

func (m *Mapper) parsePairs(v, sep string) {
	for _, pair := range strings.Split(v, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, sep, 2)
		if len(kv) == 2 {
			m.explicit[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
}

// Consumes reports whether the connector subscribes to the topic at all.
func (m *Mapper) Consumes(topic string) bool {
	for _, t := range m.topics {
		if t == topic {
			return true
		}
	}
	return m.topicsRegex != nil && m.topicsRegex.MatchString(topic)
}

// Setting names the config key that maps topics to tables.
func (m *Mapper) Setting() string { return m.setting }

// TableFor returns the table the connector writes this topic to, and how it
// knew. ok is false when the connector does not consume the topic.
func (m *Mapper) TableFor(topic string) (table string, explicit bool, ok bool) {
	if !m.Consumes(topic) {
		return "", false, false
	}
	if t, ok := m.explicit[topic]; ok {
		return t, true, true
	}
	switch m.Kind {
	case Iceberg:
		for _, r := range m.routes {
			if r.re.MatchString(topic) {
				return r.table, true, true
			}
		}
		if m.single != "" {
			return m.single, true, true
		}
		return lastSegment(topic), false, true
	case Snowflake:
		// Snowflake derives the table from the topic, replacing every
		// character that is not a letter, digit or underscore with an
		// underscore, and a leading digit gets an underscore too.
		return snowflakeName(topic), false, true
	case BigQuery:
		// The wepay connector's sanitizeTopics=true replaces invalid
		// characters with underscores; with it off the topic must already
		// be a valid table name, which a dotted Debezium topic is not.
		return strings.NewReplacer(".", "_", "-", "_").Replace(topic), false, true
	default:
		return topic, false, true
	}
}

func lastSegment(topic string) string {
	if i := strings.LastIndex(topic, "."); i >= 0 {
		return topic[i+1:]
	}
	return topic
}

func snowflakeName(topic string) string {
	var b strings.Builder
	for i, c := range topic {
		switch {
		case c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'):
			b.WriteRune(c)
		case c >= '0' && c <= '9':
			if i == 0 {
				b.WriteByte('_')
			}
			b.WriteRune(c)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// Matches reports whether a declared sink table name refers to the table
// the connector would write to: the last segment of each is compared
// case-insensitively, since every one of these warehouses folds unquoted
// identifiers and the DDL may or may not qualify the name.
func Matches(declared, mapped string) bool {
	return strings.EqualFold(lastSegment(declared), lastSegment(mapped))
}
