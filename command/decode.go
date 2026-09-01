package command

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	wire "github.com/GoCraft-MC/gocraft-abi/abi/v1/wire"
)

const CommandWireVersion = 1

// DecodeTree validates and decodes the generated commands.pb payload.
//
// Parsing is the generated codec's job; this turns the wire message into the
// neutral tree everything else works on. Generated types never leave this file:
// a host that passed a wire.CommandNode around would be a host coupled to the
// codec rather than to the format.
//
// Size is bounded before this point — a bundle entry is read under a 4 MiB cap,
// and the protobuf runtime refuses deeply nested messages on its own. The node
// and depth limits below are enforced again during conversion, because they
// bound what a reader will hold in memory rather than what the format allows.
func DecodeTree(data []byte) (Root, error) {
	var tree wire.CommandTree
	if err := proto.Unmarshal(data, &tree); err != nil {
		return Root{}, fmt.Errorf("command tree: %w", err)
	}
	if tree.GetVersion() != CommandWireVersion {
		return Root{}, fmt.Errorf("command tree: wire version %d is unsupported", tree.GetVersion())
	}
	root := Root{}
	nodeCount := 0
	for _, child := range tree.GetChildren() {
		node, err := convertNode(child, 1, &nodeCount)
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

const (
	maximumCommandNodes = 4096
	maximumCommandDepth = 64
)

// convertNode turns one wire node into the neutral tree, rejecting anything the
// schema allows but the server does not: a literal carrying argument fields, an
// argument carrying a permission, an unknown argument type.
//
// The schema cannot express those rules — protobuf has no way to say "field 4
// is only valid when field 1 is COMMAND_NODE_KIND_ARGUMENT" — so they are
// checked here, on the way in, and never trusted from the bundle.
func convertNode(node *wire.CommandNode, depth int, count *int) (Node, error) {
	*count++
	if *count > maximumCommandNodes || depth > maximumCommandDepth {
		return nil, fmt.Errorf("command tree: size limit exceeded")
	}
	children := make([]Node, 0, len(node.GetChildren()))
	for _, child := range node.GetChildren() {
		converted, err := convertNode(child, depth+1, count)
		if err != nil {
			return nil, err
		}
		children = append(children, converted)
	}
	if len(children) == 0 {
		children = nil
	}
	name := node.GetName()
	switch node.GetKind() {
	case wire.CommandNodeKind_COMMAND_NODE_KIND_LITERAL:
		if node.GetArgumentType() != wire.CommandArgumentType_COMMAND_ARGUMENT_TYPE_UNSPECIFIED ||
			len(node.GetEnumValues()) != 0 || node.GetCustomType() != "" ||
			node.IntegerMin != nil || node.IntegerMax != nil ||
			node.DecimalMin != nil || node.DecimalMax != nil {
			return nil, fmt.Errorf("command literal %q contains argument fields", name)
		}
		return Literal{
			Name: name, Permission: node.GetPermission(),
			Children: children, Exec: ExecID(node.GetExecutor()),
		}, nil
	case wire.CommandNodeKind_COMMAND_NODE_KIND_ARGUMENT:
		if node.GetPermission() != "" {
			return nil, fmt.Errorf("command argument %q contains a permission", name)
		}
		argumentType := node.GetArgumentType()
		if argumentType > wire.CommandArgumentType(ArgCustom) {
			return nil, fmt.Errorf("command argument %q has invalid type %d", name, argumentType)
		}
		return Argument{
			Name: name, Type: ArgType(argumentType), Enum: node.GetEnumValues(),
			CustomType: node.GetCustomType(),
			IntegerMin: node.IntegerMin, IntegerMax: node.IntegerMax,
			DecimalMin: node.DecimalMin, DecimalMax: node.DecimalMax,
			Children: children, Exec: ExecID(node.GetExecutor()),
		}, nil
	default:
		return nil, fmt.Errorf("command node %q has invalid kind %d", name, node.GetKind())
	}
}
