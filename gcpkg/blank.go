package gcpkg

import abi "github.com/GoCraft-MC/gocraft-abi/abi/v1"

// BlankFields is a payload of one plugin-defined event's shape, carrying
// nothing.
//
// It is what a host sends in a warm dispatch: the first event to carry values
// costs a couple of milliseconds once per process — the marshal on one side,
// the protobuf parse and the conversion on the other — and that lands on
// whichever real event happens to be first, out of a budget shared by every
// subscriber. Sending this before the runtimes go live pays it where nothing is
// waiting.
//
// Built from the manifest because that is where the layout is declared, and
// built on the host because a blank the runtime made for itself would never
// cross the socket — leaving exactly the path that carries values as cold as it
// was.
//
// Shape and never content: an empty name, a zero, a player who is nobody. The
// shape has to be right, though, since every decoder on the far side refuses a
// kind it does not expect and falls back — a blank of the wrong shape warms the
// refusal and leaves the branch a real event takes cold.
func BlankFields(definition EventDefinition, records []EventRecord) []abi.Value {
	byName := make(map[string]EventRecord, len(records))
	for _, record := range records {
		byName[record.Name] = record
	}
	fields := make([]abi.Value, 0, len(definition.Fields))
	for _, field := range definition.Fields {
		fields = append(fields, blankField(field.Type, byName, 0))
	}
	return fields
}

// maximumBlankDepth stops a record that reaches itself from recursing forever.
//
// The manifest decoder already refuses one, so this cannot be reached by a
// bundle that loaded — it is here because a warm-up is not worth a stack
// overflow if that check is ever relaxed, and because the value it returns
// (an empty list) is a shape rather than a crash.
const maximumBlankDepth = 8

func blankField(declared string, records map[string]EventRecord, depth int) abi.Value {
	parsed, ok := ParseFieldType(declared)
	if !ok || depth > maximumBlankDepth {
		return abi.List()
	}
	element := blankElement(parsed, records, depth)
	if parsed.List {
		// One element rather than none, so the loop that reads the list runs a
		// round instead of being skipped — which is the cost being paid here.
		return abi.List(element)
	}
	return element
}

func blankElement(parsed FieldType, records map[string]EventRecord, depth int) abi.Value {
	switch parsed.Element {
	case ScalarBool:
		return abi.Bool(false)
	case ScalarInt:
		return abi.Int64(0)
	case ScalarDouble:
		return abi.Double(0)
	case ScalarString:
		return abi.String("")
	case ScalarBytes:
		return abi.Bytes(nil)
	case TypePlayerRef:
		// Sixteen bytes and two strings, the shape a PlayerRef is decoded from
		// on both sides. Fewer and the decoder reads it as nobody, which is a
		// different branch from the one a real event takes.
		return abi.List(abi.Bytes(make([]byte, 16)), abi.String(""), abi.String(""))
	}
	record, declared := records[parsed.Element]
	if !declared {
		return abi.List()
	}
	fields := make([]abi.Value, 0, len(record.Fields))
	for _, field := range record.Fields {
		fields = append(fields, blankField(field.Type, records, depth+1))
	}
	return abi.List(fields...)
}
