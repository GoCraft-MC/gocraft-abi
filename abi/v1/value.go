package abi

import "bytes"

func Bool(value bool) Value {
	return Value{Kind: ValueBool, Bool: value}
}

func Int64(value int64) Value {
	return Value{Kind: ValueInt64, Int64: value}
}

func Double(value float64) Value {
	return Value{Kind: ValueDouble, Double: value}
}

func String(value string) Value {
	return Value{Kind: ValueString, String: value}
}

func Bytes(value []byte) Value {
	return Value{Kind: ValueBytes, Bytes: append([]byte(nil), value...)}
}

func List(values ...Value) Value {
	return Value{Kind: ValueList, List: append([]Value(nil), values...)}
}

// Equal reports whether two values carry the same thing.
//
// Not ==, which does not compile on a Value: it holds a byte slice and a list,
// and neither is comparable. Not reflect.DeepEqual either — this is on the
// dispatch path, and a subscriber's changes are worked out by comparing a
// payload against itself.
//
// Bytes compare by content at any depth. That reaches further than it looks: a
// PlayerRef travels as a list whose first element is bytes, and so does any
// record carrying one, so an identity comparison would report a field nobody
// touched as changed on every single dispatch.
func Equal(left, right Value) bool {
	if left.Kind != right.Kind {
		return false
	}
	switch left.Kind {
	case ValueBool:
		return left.Bool == right.Bool
	case ValueInt64:
		return left.Int64 == right.Int64
	case ValueDouble:
		return left.Double == right.Double
	case ValueString:
		return left.String == right.String
	case ValueBytes:
		return bytes.Equal(left.Bytes, right.Bytes)
	case ValueList:
		if len(left.List) != len(right.List) {
			return false
		}
		for index := range left.List {
			if !Equal(left.List[index], right.List[index]) {
				return false
			}
		}
		return true
	default:
		return true
	}
}

// Diff records what has to change for before to become after, as mutations at
// the deepest path that differs.
//
// Deep on purpose. A subscriber changing one record inside a list should send
// that one record, not the whole list: the host applies mutations in order into
// the state the next subscriber sees, and replacing a list of a thousand
// entries because one price moved would make every later subscriber pay for it.
//
// A list whose length changed is replaced whole, because a positional path
// cannot say "an element was inserted" — and growing one is refused at the far
// end anyway, since every subscriber after would be reading a shape its own
// codec does not have.
func Diff(before, after []Value) []Mutation {
	var mutations []Mutation
	diffInto(&mutations, nil, before, after)
	return mutations
}

func diffInto(mutations *[]Mutation, path []uint32, before, after []Value) {
	if len(before) != len(after) {
		return
	}
	for index := range before {
		left, right := before[index], after[index]
		// Lists first, and without asking Equal: it would walk the children to
		// answer, and the recursion below walks them again. Descending straight
		// away compares each element once.
		if left.Kind == ValueList && right.Kind == ValueList &&
			len(left.List) == len(right.List) {
			diffInto(mutations, append(path, uint32(index)), left.List, right.List)
			continue
		}
		if Equal(left, right) {
			continue
		}
		// Built only once a difference is confirmed. Allocating a path for
		// every index visited would allocate one per element of an untouched
		// list of a thousand.
		at := make([]uint32, len(path)+1)
		copy(at, path)
		at[len(path)] = uint32(index)
		*mutations = append(*mutations, Mutation{Path: at, Value: right})
	}
}
