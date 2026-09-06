package gcpkg

import (
	"reflect"
	"testing"

	abi "github.com/GoCraft-MC/gocraft-abi/abi/v1"
)

// §10's own event, with the record it carries.
var (
	blankTier = EventRecord{Name: "fr.oreo.Tier", Fields: []EventField{
		{Name: "label", Type: "string"},
		{Name: "price", Type: "double", Mutable: true},
	}}
	blankPurchase = EventDefinition{Type: "fr.oreo.shop/purchase", Cancellable: true,
		Fields: []EventField{
			{Name: "buyer", Type: "PlayerRef"},
			{Name: "tiers", Type: "[]fr.oreo.Tier"},
			{Name: "price", Type: "double", Mutable: true},
		}}
)

// The shape has to match what a real event carries, all the way down. Every
// decoder on the far side refuses a kind it does not expect and falls back, so
// a blank of the wrong shape warms the fallback and leaves the branch a real
// event takes exactly as cold as it was.
func TestBlankFieldsHasTheShapeOfARealPayload(t *testing.T) {
	fields := BlankFields(blankPurchase, []EventRecord{blankTier})

	want := []abi.Value{
		abi.List(abi.Bytes(make([]byte, 16)), abi.String(""), abi.String("")),
		abi.List(abi.List(abi.String(""), abi.Double(0))),
		abi.Double(0),
	}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("BlankFields() = %#v,\nwant %#v", fields, want)
	}
}

// One element and not none: an empty list is a shape a loop skips, and the loop
// is what needs warming.
func TestBlankFieldsGivesAListOneElement(t *testing.T) {
	fields := BlankFields(EventDefinition{Fields: []EventField{
		{Name: "names", Type: "[]string"},
	}}, nil)

	if fields[0].Kind != abi.ValueList || len(fields[0].List) != 1 {
		t.Fatalf("BlankFields() = %#v, want a list of one", fields[0])
	}
}

// A record nobody declared is a manifest the decoder would have refused. This
// answers with a shape rather than panicking, because a warm-up is not where a
// bad manifest gets reported.
func TestBlankFieldsToleratesAnUndeclaredRecord(t *testing.T) {
	fields := BlankFields(EventDefinition{Fields: []EventField{
		{Name: "tier", Type: "fr.oreo.Missing"},
	}}, nil)

	if len(fields) != 1 {
		t.Fatalf("BlankFields() = %#v, want one field", fields)
	}
}
