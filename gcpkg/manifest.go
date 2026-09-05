// Package gcpkg is the plugin bundle format: what a build tool writes and a
// host reads back.
//
// Named after the file extension rather than "bundle", because every consumer
// already has a local variable called that and a package name it shadows is a
// package nobody can reach.
//
// It describes an archive and nothing else. Where a server puts a plugin's
// data, how it scans a drop directory, when it extracts defaults — those are
// decisions about a running host, and a build tool that had to know them would
// be a build tool coupled to one server's layout.
//
// The manifest is the contract's other half. The ABI says what crosses the
// socket once a plugin runs; this says what has to be true before it can.
package gcpkg

import "github.com/GoCraft-MC/gocraft-abi/command"

// Priority orders subscribers from earliest to latest.
type Priority int8

const (
	PriorityLowest Priority = iota
	PriorityLow
	PriorityNormal
	PriorityHigh
	PriorityHighest
	PriorityMonitor
)

// Subscription describes one event declared by a plugin manifest.
type Subscription struct {
	Event    string
	Priority Priority
}

// EventField is one positional field of a plugin-defined event.
//
// The index is the contract, never the name: the wire carries values in
// declaration order and the names stay here. Appending a field is therefore
// safe, and reordering one silently shifts every index for everyone who already
// compiled against the previous shape — §10's additive rule, and the reason a
// build tool has to compare this list against the one it wrote last time.
type EventField struct {
	Name string
	// Type as the author declared it: "double", "PlayerRef", "[]fr.oreo.Tier".
	//
	// The host never resolves it. It moves values positionally, and only the
	// two runtimes that share the type have to agree on what it means.
	// Declaring it is still what lets a build tool refuse a subscriber compiled
	// against another shape, before anything runs.
	Type string
	// Mutable reports whether a subscriber may replace this field outright.
	//
	// It says nothing about what lives inside it. `final List<Tier>` is not a
	// mutable field — nobody may swap the list — while its elements are, which
	// is what the Java author wrote in §10 and what the Lua subscriber there
	// relies on when it assigns e.tiers[1].price. So this only ever answers for
	// a path of length one; see EventDefinition.MutablePath.
	Mutable bool
}

// EventDefinition is one event type a plugin declares it can emit.
type EventDefinition struct {
	// Type is namespaced, e.g. "fr.oreo.shop/purchase". The slash is required:
	// it is what makes shadowing a native event name impossible rather than
	// merely discouraged, since no native event has one.
	Type        string
	Cancellable bool
	// FailClosed makes a subscriber's failure cancel the event rather than be
	// skipped — the on_failure = DENY of §06.
	//
	// It belongs to the event and not to the subscriber: whether losing a
	// subscriber is survivable is a fact about what the event guards, and the
	// emitter is the only party that knows it.
	FailClosed bool
	Fields     []EventField
}

// MutablePath reports whether a subscriber may write at this positional path.
//
// A path of length one addresses a declared field, and its Mutable answers. A
// deeper path addresses something inside a field, which the field's own
// mutability says nothing about: a fixed list of mutable records is the common
// case rather than an exotic one. An index past the declared layout is refused
// rather than ignored — whoever wrote there compiled against another version of
// the event, and silently dropping the write would hide that.
func (d EventDefinition) MutablePath(path []uint32) bool {
	if len(path) == 0 {
		return false
	}
	index := int(path[0])
	if index >= len(d.Fields) {
		return false
	}
	if len(path) > 1 {
		return true
	}
	return d.Fields[index].Mutable
}

// Manifest contains everything the host needs before plugin code starts.
type Manifest struct {
	ID            string
	Version       string
	APIVersion    uint32
	Runtime       string
	Entry         string
	CommandTree   string
	Permissions   []string
	Subscriptions []Subscription
	// Provides are the event types this plugin can emit. The host collects them
	// across every scanned bundle before any plugin loads, which is what lets a
	// subscription name a type whose provider has not been loaded yet.
	Provides []EventDefinition
}

// Bundle is one validated .gcpkg archive and the manifest it declares.
//
// It carries no data directory. That is where a particular server chose to put
// this plugin's files, which is a fact about the server rather than about the
// archive; a host that needs one keeps it beside this.
type Bundle struct {
	Path     string
	Manifest Manifest
	Commands *command.Root
}
