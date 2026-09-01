// Package bundle is the .gcpkg format: what a build tool writes and a host
// reads back.
//
// It describes an archive and nothing else. Where a server puts a plugin's
// data, how it scans a drop directory, when it extracts defaults — those are
// decisions about a running host, and a build tool that had to know them would
// be a build tool coupled to one server's layout.
//
// The manifest is the contract's other half. The ABI says what crosses the
// socket once a plugin runs; this says what has to be true before it can.
package bundle

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
