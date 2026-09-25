// Package smt works out what a change event's value looks like after a
// connector's single message transforms, so a sink column can be matched
// to the field that actually arrives.
//
// Only transforms whose effect on field names is documented are modelled:
// Debezium's ExtractNewRecordState (and its old name UnwrapFromEnvelope) and
// the Iceberg DebeziumTransform, which unwrap the envelope into the row, and
// Kafka Connect's Flatten, which joins nested names with a delimiter ("." by
// default). Transforms that only route (RegexRouter, TimestampRouter,
// ByLogicalTableRouter) or drop whole records (Filter) change no field name.
// Anything else that touches the value, or a modelled transform applied
// under a predicate to only some records, makes the shape unknown, and an
// unknown shape produces no finding.
package smt

import (
	"strings"

	"github.com/avison9/cdclint/internal/model"
)

// Start is the value a source connector emits before its transforms. It is
// Debezium's envelope for a Debezium connector, and for a connector that
// states it with after.state.only (Confluent's managed Postgres CDC source
// keeps the envelope when it is false and sends only the row when it is
// true). Anything else is unknown.
func Start(cfg map[string]string) model.Shape {
	switch strings.ToLower(strings.TrimSpace(cfg["after.state.only"])) {
	case "false":
		return model.Shape{Kind: model.ShapeEnvelope}
	case "true":
		return model.Shape{Kind: model.ShapeRow}
	}
	if strings.HasPrefix(cfg["connector.class"], "io.debezium.") {
		return model.Shape{Kind: model.ShapeEnvelope}
	}
	return model.Shape{}
}

// Apply walks cfg's transforms in the order transforms= lists them.
func Apply(cfg map[string]string, file string, in model.Shape) model.Shape {
	s := in
	for _, name := range strings.Split(cfg["transforms"], ",") {
		name = strings.TrimSpace(name)
		if name == "" || s.Kind == model.ShapeUnknown {
			continue
		}
		key := "transforms." + name
		typ := strings.TrimSpace(cfg[key+".type"])
		conditional := cfg[key+".predicate"] != ""
		switch {
		case isRouter(typ), strings.HasSuffix(typ, "$Key"):
			// Routes the record or changes the key; the value is untouched.
		case typ == "org.apache.kafka.connect.transforms.Filter":
			// Drops whole records; the ones that pass are unchanged.
		case isUnwrap(typ) && !conditional:
			if s.Kind == model.ShapeEnvelope {
				s = model.Shape{Kind: model.ShapeRow}
			} else {
				s = model.Shape{}
			}
		case typ == "org.apache.kafka.connect.transforms.Flatten$Value" && !conditional:
			if s.Kind == model.ShapeEnvelope {
				d := cfg[key+".delimiter"]
				if d == "" {
					d = "."
				}
				s = model.Shape{Kind: model.ShapeFlattened, Delimiter: d, Transform: key, File: file}
			}
			// A row is already flat, and a flattened envelope has nothing
			// left to flatten: either way the names do not change.
		default:
			s = model.Shape{}
		}
	}
	return s
}

func isRouter(typ string) bool {
	return strings.HasSuffix(typ, "RegexRouter") ||
		typ == "org.apache.kafka.connect.transforms.TimestampRouter" ||
		typ == "io.debezium.transforms.ByLogicalTableRouter"
}

func isUnwrap(typ string) bool {
	switch typ {
	case "io.debezium.transforms.ExtractNewRecordState",
		"io.debezium.transforms.UnwrapFromEnvelope",
		"io.tabular.iceberg.connect.transforms.DebeziumTransform",
		"org.apache.iceberg.connect.transforms.DebeziumTransform":
		return true
	}
	return false
}
