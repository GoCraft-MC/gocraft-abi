package gcpkg

import "testing"

// purchase is §10's event: a read-only player, a list whose structure is fixed
// but whose elements are not, and a price subscribers may replace.
var purchase = EventDefinition{
	Type: "fr.oreo.shop/purchase", Cancellable: true,
	Fields: []EventField{
		{Name: "player", Type: "PlayerRef", Mutable: false},
		{Name: "tiers", Type: "[]fr.oreo.Tier", Mutable: false},
		{Name: "price", Type: "double", Mutable: true},
	},
}

func TestMutablePath(t *testing.T) {
	cases := []struct {
		name string
		path []uint32
		want bool
	}{
		{name: "a mutable field", path: []uint32{2}, want: true},
		{name: "a read-only field", path: []uint32{0}, want: false},
		{name: "inside a fixed list, which is what final List<Tier> means",
			path: []uint32{1, 0, 2}, want: true},
		{name: "inside a read-only scalar is still addressed by its container",
			path: []uint32{0, 1}, want: true},
		{name: "past the declared layout", path: []uint32{3}, want: false},
		{name: "past the declared layout, deeply", path: []uint32{9, 0}, want: false},
		{name: "the whole event", path: nil, want: false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := purchase.MutablePath(test.path); got != test.want {
				t.Fatalf("MutablePath(%v) = %v, want %v", test.path, got, test.want)
			}
		})
	}
}

func TestMutablePathOnAnEventWithNoFields(t *testing.T) {
	empty := EventDefinition{Type: "fr.oreo.shop/opened"}
	if empty.MutablePath([]uint32{0}) {
		t.Fatal("MutablePath([0]) accepted a write into an event carrying nothing")
	}
}
