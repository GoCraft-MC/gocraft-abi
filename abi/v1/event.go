// Package abi defines the language-neutral GoCraft plugin ABI.
package abi

// FailurePolicy decides a cancellable event when a subscriber fails to reply.
type FailurePolicy uint8

const (
	FailureAllow FailurePolicy = iota
	FailureDeny
)

// ValueKind identifies the active field in Value.
type ValueKind uint8

const (
	ValueInvalid ValueKind = iota
	ValueBool
	ValueInt64
	ValueDouble
	ValueString
	ValueBytes
	ValueList
)

// Value is the positional value carried by events, configs, and store records.
// Only the field selected by Kind is meaningful.
type Value struct {
	Kind   ValueKind
	Bool   bool
	Int64  int64
	Double float64
	String string
	Bytes  []byte
	List   []Value
}

// Event is the generic envelope used for native and plugin-defined events.
type Event struct {
	Type      string
	TypeID    uint32
	Fields    []Value
	OnFailure FailurePolicy
}

// Mutation replaces the value at a positional path after a subscriber runs.
type Mutation struct {
	Path  []uint32
	Value Value
}

// Verdict is returned by one subscriber.
type Verdict struct {
	Cancelled bool
	Mutations []Mutation
	Effects   []HostCall
}

// EventBinding names one plugin-defined event type and the id standing in for
// it on the wire. The host assigns the id; the layout is the runtime's own.
type EventBinding struct {
	TypeID uint32
	Type   string
}

// Emission is one plugin publishing a plugin-defined event.
//
// It travels runtime → host, which is the only exchange that does. Everything
// else the socket carries answers something the host asked for.
type Emission struct {
	PluginID string
	TypeID   uint32
	Fields   []Value
}

// EmissionResult is what the subscribers did to an emitted event.
//
// Error is a fault in the emitting plugin, not in the runtime carrying it —
// an unknown type id, or a type the plugin does not provide. It is reported so
// one plugin's bug does not end the process its neighbours share.
type EmissionResult struct {
	Error     string
	Cancelled bool
	Mutations []Mutation
}
