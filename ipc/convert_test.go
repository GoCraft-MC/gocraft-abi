package ipc

import (
	"reflect"
	"strings"
	"testing"

	abi "github.com/GoCraft-MC/gocraft-abi/abi/v1"
	wire "github.com/GoCraft-MC/gocraft-abi/abi/v1/wire"
)

func TestValueSurvivesARoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		value abi.Value
	}{
		{name: "bool", value: abi.Bool(true)},
		{name: "false is not empty", value: abi.Bool(false)},
		{name: "int64", value: abi.Int64(-42)},
		{name: "double", value: abi.Double(0.015625)},
		{name: "string", value: abi.String("shop")},
		{name: "empty string", value: abi.String("")},
		{name: "bytes", value: abi.Bytes([]byte{0, 1, 255})},
		{name: "empty list", value: abi.List()},
		{name: "flat list", value: abi.List(abi.Int64(1), abi.String("two"))},
		{name: "nested list", value: abi.List(abi.List(abi.Bool(true), abi.List(abi.Int64(9))))},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := encodeValue(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := decodeValue(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decoded, tc.value) {
				t.Fatalf("round trip = %+v, want %+v", decoded, tc.value)
			}
		})
	}
}

func TestEncodeValueRejectsAnUnsetKind(t *testing.T) {
	if _, err := encodeValue(abi.Value{}); err == nil {
		t.Fatal("encodeValue() accepted a value with no kind")
	}
}

func TestDecodeValueRejectsAnEmptyMessage(t *testing.T) {
	if _, err := decodeValue(&wire.Value{}); err == nil {
		t.Fatal("decodeValue() accepted a message with no kind set")
	}
	if _, err := decodeValue(nil); err == nil {
		t.Fatal("decodeValue() accepted nil")
	}
}

// A runtime is another process, so its output is untrusted: nesting has to be
// bounded on the way in rather than recursed into until the stack gives out.
func TestValueNestingIsBounded(t *testing.T) {
	deep := abi.Int64(1)
	for range maximumValueDepth + 1 {
		deep = abi.List(deep)
	}
	if _, err := encodeValue(deep); err == nil || !strings.Contains(err.Error(), "nested deeper") {
		t.Fatalf("encodeValue() error = %v", err)
	}

	message := &wire.Value{Kind: &wire.Value_Int64Value{Int64Value: 1}}
	for range maximumValueDepth + 1 {
		message = &wire.Value{Kind: &wire.Value_ListValue{ListValue: &wire.ValueList{Values: []*wire.Value{message}}}}
	}
	if _, err := decodeValue(message); err == nil || !strings.Contains(err.Error(), "nested deeper") {
		t.Fatalf("decodeValue() error = %v", err)
	}
}

func TestEventSurvivesARoundTrip(t *testing.T) {
	event := &abi.Event{
		Type:      "block.break",
		TypeID:    0,
		OnFailure: abi.FailureDeny,
		Fields: []abi.Value{
			abi.List(abi.Bytes([]byte("uuid")), abi.String("Romuald"), abi.String("java")),
			abi.List(abi.Int64(10), abi.Int64(64), abi.Int64(-3)),
			abi.String("minecraft:stone"),
		},
	}
	encoded, err := EncodeEvent(event)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeEvent(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, event) {
		t.Fatalf("round trip = %+v, want %+v", decoded, event)
	}
}

// The Go constants and the schema numbers deliberately differ: proto3 reserves
// zero for UNSPECIFIED. The mapping is this package's job and nothing else's.
func TestFailurePolicyIsRemappedNotReinterpreted(t *testing.T) {
	if abi.FailureAllow != 0 || abi.FailureDeny != 1 {
		t.Fatalf("domain policy numbering changed: allow=%d deny=%d", abi.FailureAllow, abi.FailureDeny)
	}
	allow, err := encodeFailurePolicy(abi.FailureAllow)
	if err != nil {
		t.Fatal(err)
	}
	if allow != wire.FailurePolicy_FAILURE_POLICY_ALLOW {
		t.Fatalf("allow encoded as %v", allow)
	}
	if _, err := decodeFailurePolicy(wire.FailurePolicy_FAILURE_POLICY_UNSPECIFIED); err == nil {
		t.Fatal("decodeFailurePolicy() accepted UNSPECIFIED")
	}
}

func TestVerdictSurvivesARoundTrip(t *testing.T) {
	verdict := abi.Verdict{
		Cancelled: true,
		Mutations: []abi.Mutation{
			{Path: []uint32{2, 0}, Value: abi.Int64(11)},
			{Path: nil, Value: abi.String("replaced")},
		},
		Effects: []abi.HostCall{
			{Type: "player.message", Fields: []abi.Value{abi.String("denied")}},
			{Type: "world.teleport"},
		},
	}
	encoded, err := EncodeVerdict(verdict)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeVerdict(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, verdict) {
		t.Fatalf("round trip = %+v, want %+v", decoded, verdict)
	}
}

func TestEmptyVerdictSurvivesARoundTrip(t *testing.T) {
	encoded, err := EncodeVerdict(abi.Verdict{})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeVerdict(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, abi.Verdict{}) {
		t.Fatalf("round trip = %+v, want an empty verdict", decoded)
	}
}

func TestEmissionSurvivesARoundTrip(t *testing.T) {
	emission := abi.Emission{
		PluginID: "fr.oreo.shop", TypeID: 3,
		Fields: []abi.Value{
			abi.String("oreo"),
			abi.List(abi.Double(19.99), abi.Double(4.50)),
			abi.Double(1500),
		},
	}
	encoded, err := EncodeEmission(emission)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeEmission(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, emission) {
		t.Fatalf("round trip = %+v, want %+v", decoded, emission)
	}
}

func TestEmissionRefusesTheNativeTypeID(t *testing.T) {
	// Zero is what abi/v1 puts in Event.type_id for a native event. A plugin
	// emitting it would be indistinguishable from the host on the wire, so it
	// is refused on both sides of the socket rather than trusted on one.
	if _, err := EncodeEmission(abi.Emission{PluginID: "fr.oreo.shop"}); err == nil {
		t.Fatal("EncodeEmission() accepted type id 0")
	}
	if _, err := DecodeEmission(&wire.Emit{PluginId: "fr.oreo.shop"}); err == nil {
		t.Fatal("DecodeEmission() accepted type id 0")
	}
}

func TestEmissionRefusesAnAnonymousEmitter(t *testing.T) {
	// The host skips the emitter's own subscribers, so it has to know who that
	// is; an empty id would send a plugin its own event back.
	if _, err := EncodeEmission(abi.Emission{TypeID: 1}); err == nil {
		t.Fatal("EncodeEmission() accepted an emission with no plugin id")
	}
	if _, err := DecodeEmission(&wire.Emit{TypeId: 1}); err == nil {
		t.Fatal("DecodeEmission() accepted an emission with no plugin id")
	}
}

func TestEmissionResultSurvivesARoundTrip(t *testing.T) {
	result := abi.EmissionResult{
		Cancelled: true,
		Mutations: []abi.Mutation{
			{Path: []uint32{2}, Value: abi.Double(1200)},
			{Path: []uint32{1, 0, 2}, Value: abi.Double(15.99)},
		},
	}
	encoded, err := EncodeEmissionResult(result)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeEmissionResult(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, result) {
		t.Fatalf("round trip = %+v, want %+v", decoded, result)
	}
}

func TestFailedEmissionResultCarriesItsReason(t *testing.T) {
	encoded, err := EncodeEmissionResult(abi.EmissionResult{Error: "unknown event type id 9"})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeEmissionResult(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Error != "unknown event type id 9" || decoded.Cancelled {
		t.Fatalf("round trip = %+v, want the reason kept and nothing cancelled", decoded)
	}
}

func TestEventBindingsSurviveARoundTrip(t *testing.T) {
	bindings := []abi.EventBinding{
		{TypeID: 1, Type: "fr.oreo.shop/purchase"},
		{TypeID: 4, Type: "fr.oreo.shop/refund"},
	}
	encoded, err := EncodeEventBindings(bindings)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeEventBindings(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, bindings) {
		t.Fatalf("round trip = %+v, want %+v", decoded, bindings)
	}
}

func TestEventBindingsRefuseAnUnusableEntry(t *testing.T) {
	cases := []struct {
		name    string
		binding abi.EventBinding
	}{
		{name: "the native type id", binding: abi.EventBinding{TypeID: 0, Type: "fr.oreo.shop/purchase"}},
		{name: "a nameless binding", binding: abi.EventBinding{TypeID: 1}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := EncodeEventBindings([]abi.EventBinding{test.binding}); err == nil {
				t.Fatalf("EncodeEventBindings() accepted %s", test.name)
			}
			wired := []*wire.EventBinding{{TypeId: test.binding.TypeID, Type: test.binding.Type}}
			if _, err := DecodeEventBindings(wired); err == nil {
				t.Fatalf("DecodeEventBindings() accepted %s", test.name)
			}
		})
	}
}
