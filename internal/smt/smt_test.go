package smt

import (
	"testing"

	"github.com/avison9/cdclint/internal/model"
)

const (
	debezium = "io.debezium.connector.postgresql.PostgresConnector"
	unwrap   = "io.debezium.transforms.ExtractNewRecordState"
	flatten  = "org.apache.kafka.connect.transforms.Flatten$Value"
	router   = "org.apache.kafka.connect.transforms.RegexRouter"
)

func TestShapeAfterTransforms(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  map[string]string
		want model.Shape
	}{
		{"a Debezium connector with no transforms emits the envelope",
			map[string]string{"connector.class": debezium},
			model.Shape{Kind: model.ShapeEnvelope}},
		{"after.state.only false keeps the envelope whatever the class",
			map[string]string{"connector.class": "PostgresCdcSource", "after.state.only": "false"},
			model.Shape{Kind: model.ShapeEnvelope}},
		{"after.state.only true sends the row",
			map[string]string{"connector.class": "PostgresCdcSource", "after.state.only": "true"},
			model.Shape{Kind: model.ShapeRow}},
		{"a connector cdclint does not know is unknown",
			map[string]string{"connector.class": "PostgresCdcSource"},
			model.Shape{}},
		{"unwrap turns the envelope into the row",
			map[string]string{"connector.class": debezium, "transforms": "unwrap", "transforms.unwrap.type": unwrap},
			model.Shape{Kind: model.ShapeRow}},
		{"flatten on the envelope uses . by default",
			map[string]string{"connector.class": debezium, "transforms": "flat", "transforms.flat.type": flatten},
			model.Shape{Kind: model.ShapeFlattened, Delimiter: ".", Transform: "transforms.flat", File: "c.json"}},
		{"flatten keeps its configured delimiter, and routers around it change nothing",
			map[string]string{"connector.class": debezium, "transforms": "route, flat ,route2",
				"transforms.route.type": router, "transforms.flat.type": flatten, "transforms.flat.delimiter": "_",
				"transforms.route2.type": "io.confluent.connect.cloud.transforms.TopicRegexRouter"},
			model.Shape{Kind: model.ShapeFlattened, Delimiter: "_", Transform: "transforms.flat", File: "c.json"}},
		{"flatten after unwrap leaves the row as it is",
			map[string]string{"connector.class": debezium, "transforms": "unwrap,flat",
				"transforms.unwrap.type": unwrap, "transforms.flat.type": flatten},
			model.Shape{Kind: model.ShapeRow}},
		{"unwrap after flatten is not modelled",
			map[string]string{"connector.class": debezium, "transforms": "flat,unwrap",
				"transforms.unwrap.type": unwrap, "transforms.flat.type": flatten},
			model.Shape{}},
		{"a flatten under a predicate applies to some records only, so the shape is unknown",
			map[string]string{"connector.class": debezium, "transforms": "flat",
				"transforms.flat.type": flatten, "transforms.flat.predicate": "isOrders"},
			model.Shape{}},
		{"a transform that renames value fields makes the shape unknown",
			map[string]string{"connector.class": debezium, "transforms": "flat,rename",
				"transforms.flat.type": flatten, "transforms.rename.type": "org.apache.kafka.connect.transforms.ReplaceField$Value"},
			model.Shape{}},
		{"key and filter transforms leave the value alone",
			map[string]string{"connector.class": debezium, "transforms": "key,drop,flat",
				"transforms.key.type":  "org.apache.kafka.connect.transforms.ExtractField$Key",
				"transforms.drop.type": "org.apache.kafka.connect.transforms.Filter",
				"transforms.flat.type": flatten},
			model.Shape{Kind: model.ShapeFlattened, Delimiter: ".", Transform: "transforms.flat", File: "c.json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Apply(tc.cfg, "c.json", Start(tc.cfg)); got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestSinkTransformsApplyAfterTheSource(t *testing.T) {
	sink := map[string]string{"transforms": "unwrap", "transforms.unwrap.type": unwrap}
	if got := Apply(sink, "sink.json", model.Shape{Kind: model.ShapeEnvelope}); got.Kind != model.ShapeRow {
		t.Errorf("unwrap on the sink side: got %+v, want the row", got)
	}
	flat := map[string]string{"transforms": "f", "transforms.f.type": flatten}
	if got := Apply(flat, "sink.json", model.Shape{Kind: model.ShapeRow}); got.Kind != model.ShapeRow {
		t.Errorf("flatten on the sink side of an unwrapped row: got %+v, want the row", got)
	}
}
