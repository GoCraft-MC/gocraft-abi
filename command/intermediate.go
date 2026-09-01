package command

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// The neutral form a build tool hands over, and what turns it into a tree.
//
// Every runtime extracts its commands differently — an annotation processor
// inside javac, a Go plugin asked to describe itself, whatever a Lua one ends
// up doing — and none of them may encode a commands.pb. They all write this
// instead, and one program reads it: whoever builds the bundle. That is what
// keeps the wire format single-sourced while the extraction stays per-language.
//
// JSON rather than protobuf because the producers are compilers and build
// scripts. A protobuf runtime on an annotation processor's path, for a handful
// of fields read by one program in the same build, would be a dependency bought
// for nothing.
//
// **No executor ids.** They are minted here, once, walking the tree in
// declaration order. An id is an artefact of the tree; a compiler that invented
// its own would be a second set to keep in step, and paths — not ids — are what
// the two sides of a bundle agree on.

// IntermediateVersion is the shape this reads. A build tool newer than the
// server is a real situation, and refusing it by number beats failing on a
// field that moved.
const IntermediateVersion = 1

// maximumIntermediateSize bounds what a build tool may hand over. The file is a
// few fields per command; anything past this is a mistake or a different file.
const maximumIntermediateSize = 4 << 20

type intermediateFile struct {
	Version  int                `json:"version"`
	Commands []intermediateNode `json:"commands"`
}

type intermediateNode struct {
	Name       string             `json:"name"`
	Argument   bool               `json:"argument"`
	Kind       string             `json:"kind,omitempty"`
	Permission string             `json:"permission,omitempty"`
	Runs       bool               `json:"runs"`
	Minimum    *float64           `json:"min,omitempty"`
	Maximum    *float64           `json:"max,omitempty"`
	Options    []string           `json:"options,omitempty"`
	Custom     string             `json:"custom,omitempty"`
	Children   []intermediateNode `json:"children,omitempty"`
}

// DecodeIntermediate turns what a build tool extracted into a tree, minting the
// executor ids as it goes.
//
// The result is validated: a tree the host would refuse fails here, on the
// machine that has the source, rather than at load time on someone's server.
func DecodeIntermediate(data []byte) (Root, error) {
	if len(data) > maximumIntermediateSize {
		return Root{}, fmt.Errorf("command trees: exceeds %d bytes", maximumIntermediateSize)
	}
	var file intermediateFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	// A field this does not know is a build tool describing something this
	// cannot honour. Ignoring it would ship a tree quietly missing whatever it
	// meant.
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return Root{}, fmt.Errorf("command trees: %w", err)
	}
	if file.Version != IntermediateVersion {
		return Root{}, fmt.Errorf("command trees: version %d is unsupported, this reads %d",
			file.Version, IntermediateVersion)
	}
	if len(file.Commands) == 0 {
		return Root{}, fmt.Errorf("command trees: declares no commands")
	}

	var executor ExecID
	next := func() ExecID {
		executor++
		return executor
	}
	root := Root{}
	for _, declared := range file.Commands {
		node, err := convertIntermediate(declared, next)
		if err != nil {
			return Root{}, err
		}
		root.Children = append(root.Children, node)
	}
	if err := Validate(&root); err != nil {
		return Root{}, err
	}
	return root, nil
}

// EncodeIntermediate writes a tree in the neutral form, dropping the executor
// ids.
//
// For a runtime that builds its tree as code rather than extracting it from
// source: it declares once, hands over the same file every other runtime hands
// over, and never touches the wire format.
func EncodeIntermediate(root Root) ([]byte, error) {
	if err := Validate(&root); err != nil {
		return nil, err
	}
	file := intermediateFile{Version: IntermediateVersion}
	for _, child := range root.Children {
		node, err := describeIntermediate(child)
		if err != nil {
			return nil, err
		}
		file.Commands = append(file.Commands, node)
	}
	encoded, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("command trees: %w", err)
	}
	return append(encoded, '\n'), nil
}

func convertIntermediate(node intermediateNode, next func() ExecID) (Node, error) {
	children := make([]Node, 0, len(node.Children))
	for _, child := range node.Children {
		converted, err := convertIntermediate(child, next)
		if err != nil {
			return nil, err
		}
		children = append(children, converted)
	}
	if len(children) == 0 {
		children = nil
	}
	var exec ExecID
	if node.Runs {
		exec = next()
	}
	if !node.Argument {
		if node.Kind != "" || node.Options != nil || node.Minimum != nil || node.Maximum != nil {
			return nil, fmt.Errorf("literal %q carries argument fields", node.Name)
		}
		return Literal{
			Name: node.Name, Permission: node.Permission, Exec: exec, Children: children,
		}, nil
	}
	if node.Permission != "" {
		return nil, fmt.Errorf("argument %q carries a permission", node.Name)
	}
	kind, err := argumentKind(node.Kind)
	if err != nil {
		return nil, fmt.Errorf("argument %q: %w", node.Name, err)
	}
	argument := Argument{
		Name: node.Name, Type: kind, Enum: node.Options,
		CustomType: node.Custom, Exec: exec, Children: children,
	}
	switch kind {
	case ArgInteger:
		argument.IntegerMin, argument.IntegerMax = wholeBound(node.Minimum), wholeBound(node.Maximum)
	case ArgDecimal:
		argument.DecimalMin, argument.DecimalMax = node.Minimum, node.Maximum
	default:
		if node.Minimum != nil || node.Maximum != nil {
			return nil, fmt.Errorf("argument %q is %s and carries a range", node.Name, node.Kind)
		}
	}
	return argument, nil
}

func describeIntermediate(node Node) (intermediateNode, error) {
	switch typed := node.(type) {
	case Literal:
		described := intermediateNode{
			Name: typed.Name, Permission: typed.Permission, Runs: typed.Exec != 0,
		}
		children, err := describeChildren(typed.Children)
		if err != nil {
			return intermediateNode{}, err
		}
		described.Children = children
		return described, nil
	case Argument:
		kind, err := kindName(typed.Type)
		if err != nil {
			return intermediateNode{}, fmt.Errorf("argument %q: %w", typed.Name, err)
		}
		described := intermediateNode{
			Name: typed.Name, Argument: true, Kind: kind, Runs: typed.Exec != 0,
			Options: typed.Enum, Custom: typed.CustomType,
		}
		switch typed.Type {
		case ArgInteger:
			described.Minimum, described.Maximum = decimalBound(typed.IntegerMin), decimalBound(typed.IntegerMax)
		case ArgDecimal:
			described.Minimum, described.Maximum = typed.DecimalMin, typed.DecimalMax
		}
		children, err := describeChildren(typed.Children)
		if err != nil {
			return intermediateNode{}, err
		}
		described.Children = children
		return described, nil
	}
	return intermediateNode{}, fmt.Errorf("command trees: cannot describe %T", node)
}

func describeChildren(nodes []Node) ([]intermediateNode, error) {
	if len(nodes) == 0 {
		return nil, nil
	}
	described := make([]intermediateNode, 0, len(nodes))
	for _, node := range nodes {
		converted, err := describeIntermediate(node)
		if err != nil {
			return nil, err
		}
		described = append(described, converted)
	}
	return described, nil
}

// argumentKind reads the neutral spelling, and kindName writes it.
//
// Named rather than numbered on the wire between two build tools: a number
// would mean every extractor and this program agreeing on an enum order none of
// them declares.
func argumentKind(kind string) (ArgType, error) {
	for named, typed := range kindNames {
		if named == kind {
			return typed, nil
		}
	}
	if kind == "" {
		return 0, fmt.Errorf("no kind")
	}
	return 0, fmt.Errorf("unknown kind %q", kind)
}

func kindName(kind ArgType) (string, error) {
	for named, typed := range kindNames {
		if typed == kind {
			return named, nil
		}
	}
	return "", fmt.Errorf("unknown type %d", kind)
}

var kindNames = map[string]ArgType{
	"integer":     ArgInteger,
	"decimal":     ArgDecimal,
	"string":      ArgString,
	"greedy":      ArgGreedy,
	"player":      ArgPlayer,
	"block_pos":   ArgBlockPos,
	"block_state": ArgBlockState,
	"item":        ArgItem,
	"duration":    ArgDuration,
	"enum":        ArgEnum,
	"custom":      ArgCustom,
}

func wholeBound(value *float64) *int64 {
	if value == nil {
		return nil
	}
	whole := int64(*value)
	return &whole
}

func decimalBound(value *int64) *float64 {
	if value == nil {
		return nil
	}
	decimal := float64(*value)
	return &decimal
}
