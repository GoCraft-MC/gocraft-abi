package gcpkg

import (
	"fmt"
	"sort"
	"strings"
)

// The vocabulary a plugin-defined event field may hold.
//
// Closed, and matching what the wire carries: abi.Value has exactly these kinds
// plus a list. A type outside this set is refused at build time rather than
// carried as a name nobody resolves — see EventField.Type for why that
// distinction stopped being free once fields could be records.
const (
	ScalarBool   = "bool"
	ScalarInt    = "int"
	ScalarDouble = "double"
	ScalarString = "string"
	ScalarBytes  = "bytes"

	// TypePlayerRef is the one vocabulary type a plugin-defined event may
	// carry, and the only name here that is not a wire kind.
	//
	// Both runtimes already have a hand-written PlayerRef, and both bind it to
	// the dispatch as they decode it — so an event that carries one hands its
	// subscriber somebody it can answer, rather than sixteen bytes it has to
	// turn into a handle itself. Spelling it out is the difference between an
	// event about a player and an event carrying an id that happens to be one.
	//
	// The others are not offered. BlockPos and Block describe world state, and
	// §10's rule is that an event carrying a live object is carrying
	// implementation instead of a fact; a plugin that means a position can
	// declare a record of three ints and say what they are.
	TypePlayerRef = "PlayerRef"
)

// listPrefix marks a repeated field. One level: a list of lists has no author
// asking for it yet, and allowing it now would mean deciding how deep a
// mutation path may reach before anybody has written one.
const listPrefix = "[]"

func scalar(name string) bool {
	switch name {
	case ScalarBool, ScalarInt, ScalarDouble, ScalarString, ScalarBytes, TypePlayerRef:
		return true
	}
	return false
}

// FieldType is one field's declared type, taken apart.
type FieldType struct {
	// List reports the [] prefix.
	List bool
	// Element is what the field holds, or what its list holds: a scalar name or
	// a record name.
	Element string
	// Record reports whether Element names a record rather than a scalar.
	Record bool
}

// ParseFieldType reads a declared type, or reports that it is not one.
//
// Nothing here resolves the record: whether the name is declared is a question
// about the whole manifest, and this answers only about the string.
func ParseFieldType(declared string) (FieldType, bool) {
	element := strings.TrimSpace(declared)
	if element == "" {
		return FieldType{}, false
	}
	parsed := FieldType{}
	if strings.HasPrefix(element, listPrefix) {
		parsed.List = true
		element = element[len(listPrefix):]
	}
	// After one prefix, another means a list of lists.
	if element == "" || strings.HasPrefix(element, listPrefix) {
		return FieldType{}, false
	}
	parsed.Element = element
	if scalar(element) {
		return parsed, true
	}
	// Anything else has to be a record name. A slash would make it an event
	// type, which is a different vocabulary and not something a field holds.
	if !validRecordName(element) {
		return FieldType{}, false
	}
	parsed.Record = true
	return parsed, true
}

// validRecordName accepts a dotted name with the case a type is written in.
//
// Not validDottedName, which is lowercase-only because it guards event types
// and plugin ids — names that are addresses. A record is a type, and every
// language this contract serves capitalises one. Refusing Tier would mean
// forcing an author to spell their own class differently here.
//
// A slash is refused: that form belongs to event types, and a record is not one.
func validRecordName(name string) bool {
	if name == "" || name[0] == '.' || name[len(name)-1] == '.' {
		return false
	}
	if name[0] >= '0' && name[0] <= '9' {
		return false
	}
	for _, character := range name {
		switch {
		case character >= 'a' && character <= 'z',
			character >= 'A' && character <= 'Z',
			character >= '0' && character <= '9',
			character == '.', character == '_':
		default:
			return false
		}
	}
	return !strings.Contains(name, "..")
}

// validateRecords checks the record table on its own, then that every reference
// resolves and no record contains itself.
func validateRecords(manifest Manifest) error {
	byName := make(map[string]EventRecord, len(manifest.Records))
	for _, record := range manifest.Records {
		if !validRecordName(record.Name) {
			return fmt.Errorf("plugin %s: invalid record name %q, want a dotted name like fr.oreo.Tier",
				manifest.ID, record.Name)
		}
		if _, duplicate := byName[record.Name]; duplicate {
			return fmt.Errorf("plugin %s: duplicate record %s", manifest.ID, record.Name)
		}
		if len(record.Fields) == 0 {
			// A record with no fields encodes to an empty list and tells a
			// subscriber nothing. It is a mistake worth naming rather than a
			// shape worth supporting.
			return fmt.Errorf("plugin %s: record %s carries no fields", manifest.ID, record.Name)
		}
		if err := validateFields(manifest.ID, "record "+record.Name, record.Fields); err != nil {
			return err
		}
		byName[record.Name] = record
	}
	for _, record := range manifest.Records {
		if err := resolveFields(manifest.ID, "record "+record.Name, record.Fields, byName); err != nil {
			return err
		}
	}
	return detectCycles(manifest.ID, byName)
}

// validateFields checks names and type syntax for one field list.
func validateFields(pluginID, owner string, fields []EventField) error {
	names := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if !validFieldName(field.Name) {
			return fmt.Errorf("plugin %s: %s: invalid field name %q", pluginID, owner, field.Name)
		}
		if _, duplicate := names[field.Name]; duplicate {
			return fmt.Errorf("plugin %s: %s: duplicate field %s", pluginID, owner, field.Name)
		}
		names[field.Name] = struct{}{}
		if _, ok := ParseFieldType(field.Type); !ok {
			return fmt.Errorf("plugin %s: %s: field %s has type %q, want one of "+
				"bool, int, double, string, bytes, PlayerRef, a declared record, "+
				"or [] before any of them", pluginID, owner, field.Name, field.Type)
		}
	}
	return nil
}

// resolveFields checks that every record a field names is declared.
func resolveFields(pluginID, owner string, fields []EventField, byName map[string]EventRecord) error {
	for _, field := range fields {
		parsed, _ := ParseFieldType(field.Type)
		if !parsed.Record {
			continue
		}
		if _, declared := byName[parsed.Element]; !declared {
			return fmt.Errorf("plugin %s: %s: field %s names record %s, which no [[events.types]] declares",
				pluginID, owner, field.Name, parsed.Element)
		}
	}
	return nil
}

// detectCycles refuses a record that contains itself.
//
// The wire has no pointers: a payload is a finite list of values, so a record
// reaching itself is not a shape that could be encoded at all. Said here rather
// than discovered as a codec recursing until the stack runs out.
func detectCycles(pluginID string, byName map[string]EventRecord) error {
	const (
		visiting = 1
		done     = 2
	)
	state := make(map[string]int, len(byName))
	var walk func(name string, path []string) error
	walk = func(name string, path []string) error {
		switch state[name] {
		case done:
			return nil
		case visiting:
			return fmt.Errorf("plugin %s: record %s contains itself, through %s",
				pluginID, name, strings.Join(append(path, name), " -> "))
		}
		state[name] = visiting
		for _, field := range byName[name].Fields {
			parsed, _ := ParseFieldType(field.Type)
			if !parsed.Record {
				continue
			}
			if err := walk(parsed.Element, append(path, name)); err != nil {
				return err
			}
		}
		state[name] = done
		return nil
	}
	// Sorted so the same manifest always names the same record in the message.
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := walk(name, nil); err != nil {
			return err
		}
	}
	return nil
}
