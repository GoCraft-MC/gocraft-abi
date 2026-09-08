package abi

import "fmt"

// ApplyPath writes one mutation into a positional value list and returns the
// result.
//
// It lives in the contract because both ends apply the same paths: the host
// carries an event from one subscriber to the next, and the emitting runtime
// replays what came back into the object it published. Written twice, the two
// would agree until one of them decided differently about an index one past the
// end.
//
// The input is never modified. Only the values along the path are copied — the
// rest of the list is shared with the original, because a plugin-defined event
// can carry a list of a thousand records and re-copying it per subscriber is
// the kind of cost that turns a 2 ms budget into a missed tick.
//
// An index past the end of a list is an error rather than a silent extension.
// A subscriber writing there compiled against a different version of the event,
// and growing the list to fit would hand every later subscriber a shape its own
// codec cannot read.
func ApplyPath(values []Value, mutation Mutation) ([]Value, error) {
	if len(mutation.Path) == 0 {
		return nil, fmt.Errorf("abi: mutation has no path")
	}
	return applyPath(values, mutation.Path, mutation.Value)
}

func applyPath(values []Value, path []uint32, replacement Value) ([]Value, error) {
	index := int(path[0])
	if index >= len(values) {
		return nil, fmt.Errorf("abi: mutation index %d is past the end of %d values", index, len(values))
	}
	// Copy-on-write: one new backing array per level actually descended into.
	updated := make([]Value, len(values))
	copy(updated, values)
	if len(path) == 1 {
		updated[index] = replacement
		return updated, nil
	}
	if updated[index].Kind != ValueList {
		return nil, fmt.Errorf("abi: mutation descends into a %s at index %d, which holds no values",
			updated[index].Kind, index)
	}
	nested, err := applyPath(updated[index].List, path[1:], replacement)
	if err != nil {
		return nil, err
	}
	updated[index] = Value{Kind: ValueList, List: nested}
	return updated, nil
}

// String names a kind in an error an author has to act on. Without it the
// message reports a number and leaves them counting constants.
func (k ValueKind) String() string {
	switch k {
	case ValueBool:
		return "bool"
	case ValueInt64:
		return "int64"
	case ValueDouble:
		return "double"
	case ValueString:
		return "string"
	case ValueBytes:
		return "bytes"
	case ValueList:
		return "list"
	}
	return "invalid value"
}
