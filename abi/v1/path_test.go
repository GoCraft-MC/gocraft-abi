package abi

import (
	"reflect"
	"strings"
	"testing"
)

// purchase is §10's event: a player, a list of tiers each carrying a price, and
// a price of its own.
func purchase() []Value {
	return []Value{
		String("oreo"),
		List(List(String("gold"), Double(19.99)), List(String("iron"), Double(4.50))),
		Double(1500),
	}
}

func TestApplyPathReplacesAField(t *testing.T) {
	updated, err := ApplyPath(purchase(), Mutation{Path: []uint32{2}, Value: Double(1200)})
	if err != nil {
		t.Fatal(err)
	}
	if updated[2].Double != 1200 {
		t.Fatalf("ApplyPath() = %+v, want the price replaced", updated[2])
	}
}

// The deep write §10's Lua subscriber makes: e.tiers[1].price = ...
func TestApplyPathReachesInsideAList(t *testing.T) {
	updated, err := ApplyPath(purchase(), Mutation{Path: []uint32{1, 0, 1}, Value: Double(15.99)})
	if err != nil {
		t.Fatal(err)
	}
	if got := updated[1].List[0].List[1].Double; got != 15.99 {
		t.Fatalf("ApplyPath() left the nested price at %v", got)
	}
	if got := updated[1].List[1].List[1].Double; got != 4.50 {
		t.Fatalf("ApplyPath() disturbed a sibling: %v", got)
	}
}

// A subscriber runs against the state the previous one left, so the previous
// state must not change under it — nor may the emitter's own copy.
func TestApplyPathNeverTouchesTheInput(t *testing.T) {
	before := purchase()
	if _, err := ApplyPath(before, Mutation{Path: []uint32{1, 0, 1}, Value: Double(15.99)}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, purchase()) {
		t.Fatalf("ApplyPath() modified its input: %+v", before)
	}
}

func TestApplyPathRefusesWhatItCannotWrite(t *testing.T) {
	cases := []struct {
		name string
		path []uint32
		want string
	}{
		{name: "no path at all", path: nil, want: "no path"},
		{name: "past the end", path: []uint32{3}, want: "past the end"},
		{name: "past the end of a nested list", path: []uint32{1, 7}, want: "past the end"},
		{name: "into a scalar", path: []uint32{2, 0}, want: "holds no values"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := ApplyPath(purchase(), Mutation{Path: test.path, Value: Double(1)})
			if err == nil {
				t.Fatalf("ApplyPath() accepted %s", test.name)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ApplyPath() error = %v, want it to mention %q", err, test.want)
			}
		})
	}
}

func TestValueKindNamesItself(t *testing.T) {
	if got := ValueDouble.String(); got != "double" {
		t.Fatalf("ValueDouble.String() = %q", got)
	}
	if got := ValueInvalid.String(); got != "invalid value" {
		t.Fatalf("ValueInvalid.String() = %q", got)
	}
}
