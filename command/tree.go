// Package command is the command tree a plugin ships and a host renders.
//
// It is a format, not a dispatcher. What lives here is the shape of a tree, the
// rules a valid one obeys, and the two codecs that put it in a bundle and take
// it back out — everything a build tool and a server must agree on without
// talking to each other.
//
// What does not live here is anything only a host does: resolving a typed line
// against a permission-pruned view, keeping a registry, invoking a handler.
// That needs a notion of who is speaking, and a plugin's build has no such
// notion.
//
// The point of sharing this rather than describing it twice is that a bundle
// cannot ship a tree the server would refuse: the build validates with the same
// code, so the failure lands on the machine that has the source.
package command

// ExecID names one executable node within a tree. It is an artefact of the
// tree, minted by whoever assembles it, and means nothing outside it.
type ExecID uint32

type ArgType uint8

const (
	ArgInteger ArgType = iota + 1
	ArgDecimal
	ArgString
	ArgGreedy
	ArgPlayer
	ArgBlockPos
	ArgBlockState
	ArgItem
	ArgDuration
	ArgEnum
	ArgCustom
)

// Node is sealed so every renderer sees the complete command IR.
type Node interface{ isNode() }

type Root struct {
	Children []Node
}

func (Root) isNode() {}

type Literal struct {
	Name       string
	Permission string
	Children   []Node
	Exec       ExecID
}

func (Literal) isNode() {}

type Argument struct {
	Name       string
	Type       ArgType
	Enum       []string
	CustomType string
	IntegerMin *int64
	IntegerMax *int64
	DecimalMin *float64
	DecimalMax *float64
	Children   []Node
	Exec       ExecID
}

func (Argument) isNode() {}
