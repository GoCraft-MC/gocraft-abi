package abi

import "testing"

// Bytes at any depth, because a PlayerRef travels as a list whose first element
// is bytes and so does any record carrying one. An identity comparison reported
// a field nobody touched as changed on every dispatch.
func TestEqualComparesBytesByContent(t *testing.T) {
	uuid := make([]byte, 16)
	same := make([]byte, 16)
	player := func(raw []byte) Value {
		return List(Bytes(raw), String("oreo"), String("java"))
	}
	if !Equal(player(uuid), player(same)) {
		t.Fatal("two players with the same uuid compared unequal")
	}
	other := make([]byte, 16)
	other[0] = 1
	if Equal(player(uuid), player(other)) {
		t.Fatal("two players with different uuids compared equal")
	}
}

func TestEqualSeparatesKindsAndLengths(t *testing.T) {
	if Equal(Int64(1), Double(1)) {
		t.Fatal("an int compared equal to a decimal")
	}
	if Equal(List(Int64(1)), List(Int64(1), Int64(2))) {
		t.Fatal("lists of different lengths compared equal")
	}
	if !Equal(List(), List()) {
		t.Fatal("two empty lists compared unequal")
	}
}

// The point of the whole thing: one record inside a list moves, and what
// travels is that record — not the list it sits in.
func TestDiffReachesTheDeepestChange(t *testing.T) {
	tier := func(label string, price float64) Value {
		return List(String(label), Double(price))
	}
	before := []Value{String("oreo"), List(tier("gold", 19.99), tier("iron", 4.50))}
	after := []Value{String("oreo"), List(tier("gold", 15.99), tier("iron", 4.50))}

	mutations := Diff(before, after)
	if len(mutations) != 1 {
		t.Fatalf("Diff() = %+v, want one mutation", mutations)
	}
	got := mutations[0]
	if len(got.Path) != 3 || got.Path[0] != 1 || got.Path[1] != 0 || got.Path[2] != 1 {
		t.Fatalf("path = %v, want [1 0 1] — the price of the first tier", got.Path)
	}
	if got.Value.Double != 15.99 {
		t.Fatalf("value = %+v", got.Value)
	}
}

func TestDiffSaysNothingAboutWhatDidNotMove(t *testing.T) {
	fields := []Value{String("oreo"), Bytes(make([]byte, 16)), Double(1500)}
	unchanged := []Value{String("oreo"), Bytes(make([]byte, 16)), Double(1500)}
	if mutations := Diff(fields, unchanged); len(mutations) != 0 {
		t.Fatalf("Diff() = %+v, want nothing", mutations)
	}
}

// A positional path cannot say "an element was inserted", and growing a list is
// refused at the far end anyway — every later subscriber would be reading a
// shape its own codec does not have.
func TestDiffReplacesAListThatChangedLength(t *testing.T) {
	before := []Value{List(Int64(1))}
	after := []Value{List(Int64(1), Int64(2))}
	mutations := Diff(before, after)
	if len(mutations) != 1 || len(mutations[0].Path) != 1 {
		t.Fatalf("Diff() = %+v, want the whole field replaced", mutations)
	}
}
