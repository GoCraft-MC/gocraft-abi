package gcpkg

import (
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	// ManifestFileName is the manifest at the root of a bundle, and of the
	// source directory a bundle is built from.
	ManifestFileName = "plugin.toml"

	CurrentAPIVersion   = 1
	maximumManifestSize = 1 << 20
)

type manifestFile struct {
	ID         string             `toml:"id"`
	Version    string             `toml:"version"`
	APIVersion uint32             `toml:"api"`
	Runtime    string             `toml:"runtime"`
	Entry      string             `toml:"entry"`
	Subscribe  []subscriptionFile `toml:"subscribe"`
	Commands   struct {
		Tree string `toml:"tree"`
	} `toml:"commands"`
	Events struct {
		Types    []recordFile        `toml:"types"`
		Provides []providedEventFile `toml:"provides"`
	} `toml:"events"`
}

// subscriptionFile is one [[subscribe]] block.
//
// An array of tables rather than a list of names, because a subscription has
// more to say than which event it is for: at what priority it runs, and which
// permission nodes it needs answered. A flat list could express neither, which
// is why priorities were unreachable and why every node a plugin declared
// travelled with every event it subscribed to.
type subscriptionFile struct {
	Event       string   `toml:"event"`
	Priority    string   `toml:"priority"`
	Permissions []string `toml:"perms"`
}

type recordFile struct {
	Name   string           `toml:"name"`
	Fields []eventFieldFile `toml:"fields"`
}

type providedEventFile struct {
	Type        string           `toml:"type"`
	Cancellable bool             `toml:"cancellable"`
	FailClosed  bool             `toml:"fail_closed"`
	Fields      []eventFieldFile `toml:"fields"`
}

type eventFieldFile struct {
	Name    string `toml:"name"`
	Type    string `toml:"type"`
	Mutable bool   `toml:"mutable"`
}

// DecodeManifest reads and validates one plugin.toml. It is the only manifest
// parser in the project: the host calls it when opening a bundle and the CLI
// calls it when building one, so build-time and load-time validation cannot
// drift apart.
func DecodeManifest(reader io.Reader) (Manifest, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maximumManifestSize+1))
	if err != nil {
		return Manifest{}, fmt.Errorf("read %s: %w", ManifestFileName, err)
	}
	if len(data) > maximumManifestSize {
		return Manifest{}, fmt.Errorf("%s exceeds %d bytes", ManifestFileName, maximumManifestSize)
	}
	var file manifestFile
	decoder := toml.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return Manifest{}, decodeFailure(err)
	}
	manifest := Manifest{
		ID: file.ID, Version: file.Version, APIVersion: file.APIVersion,
		Runtime: file.Runtime, Entry: file.Entry, CommandTree: file.Commands.Tree,
	}
	seenNode := map[string]struct{}{}
	for _, declared := range file.Subscribe {
		priority, err := parsePriority(declared.Priority)
		if err != nil {
			return Manifest{}, fmt.Errorf("plugin %s: subscription %s: %w",
				file.ID, declared.Event, err)
		}
		manifest.Subscriptions = append(manifest.Subscriptions, Subscription{
			Event: declared.Event, Priority: priority,
			Permissions: append([]string(nil), declared.Permissions...),
		})
		// The union, for the command path: a command handler asking can() is
		// not inside any subscription, so it sees every node the plugin
		// declared anywhere.
		for _, node := range declared.Permissions {
			if _, done := seenNode[node]; done {
				continue
			}
			seenNode[node] = struct{}{}
			manifest.Permissions = append(manifest.Permissions, node)
		}
	}
	for _, declared := range file.Events.Types {
		record := EventRecord{Name: declared.Name, Fields: readFields(declared.Fields)}
		manifest.Records = append(manifest.Records, record)
	}
	for _, provided := range file.Events.Provides {
		definition := EventDefinition{
			Type: provided.Type, Cancellable: provided.Cancellable,
			FailClosed: provided.FailClosed,
			Fields:     make([]EventField, 0, len(provided.Fields)),
		}
		definition.Fields = readFields(provided.Fields)
		manifest.Provides = append(manifest.Provides, definition)
	}
	if err := ValidateManifest(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// decodeFailure names the offending key and the line it sits on. The library's
// own text for an unknown field — "fields in the document are missing in the
// target struct" — tells a plugin author nothing about what to fix.
func decodeFailure(err error) error {
	var missing *toml.StrictMissingError
	if errors.As(err, &missing) {
		reported := make([]string, 0, len(missing.Errors))
		for index := range missing.Errors {
			field := &missing.Errors[index]
			line, _ := field.Position()
			reported = append(reported, fmt.Sprintf("%s:%d: unknown field %q", ManifestFileName, line, strings.Join(field.Key(), ".")))
		}
		return errors.New(strings.Join(reported, "; "))
	}
	var decode *toml.DecodeError
	if errors.As(err, &decode) {
		line, column := decode.Position()
		return fmt.Errorf("%s:%d:%d: %s", ManifestFileName, line, column, decode.Error())
	}
	return fmt.Errorf("decode %s: %w", ManifestFileName, err)
}

// ValidateManifest reports whether a manifest is well formed.
//
// DecodeManifest runs it, so a manifest read from an archive has already
// passed. It is exported for the other way in: a host that assembles a
// manifest itself, or a test that writes one by hand, asks the same question
// rather than a second opinion of it.
func ValidateManifest(manifest Manifest) error {
	if !validPluginID(manifest.ID) {
		return fmt.Errorf("plugin manifest: invalid id %q", manifest.ID)
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return fmt.Errorf("plugin %s: version is required", manifest.ID)
	}
	if manifest.APIVersion != CurrentAPIVersion {
		return fmt.Errorf("plugin %s: API %d is unsupported, host uses %d", manifest.ID, manifest.APIVersion, CurrentAPIVersion)
	}
	if strings.TrimSpace(manifest.Runtime) == "" {
		return fmt.Errorf("plugin %s: runtime is required", manifest.ID)
	}
	if manifest.CommandTree != "" && !validBundleReference(manifest.CommandTree) {
		return fmt.Errorf("plugin %s: invalid command tree path %q", manifest.ID, manifest.CommandTree)
	}
	seen := make(map[string]struct{}, len(manifest.Subscriptions))
	for _, subscription := range manifest.Subscriptions {
		if strings.TrimSpace(subscription.Event) == "" {
			return fmt.Errorf("plugin %s: empty event subscription", manifest.ID)
		}
		if _, duplicate := seen[subscription.Event]; duplicate {
			return fmt.Errorf("plugin %s: duplicate subscription to %s", manifest.ID, subscription.Event)
		}
		seen[subscription.Event] = struct{}{}
		// Per subscription, because that is where the nodes are declared now.
		// Two subscriptions naming the same node is not a mistake — two events
		// may both need it answered — but naming it twice in one is.
		permissions := make(map[string]struct{}, len(subscription.Permissions))
		for _, permission := range subscription.Permissions {
			if strings.TrimSpace(permission) == "" {
				return fmt.Errorf("plugin %s: empty subscribed permission", manifest.ID)
			}
			if _, duplicate := permissions[permission]; duplicate {
				return fmt.Errorf("plugin %s: duplicate subscribed permission %s",
					manifest.ID, permission)
			}
			permissions[permission] = struct{}{}
		}
	}
	if err := validateRecords(manifest); err != nil {
		return err
	}
	return validateProvides(manifest)
}

// readFields converts one declared field list, without judging it. What a type
// may say is validateFields; whether a record it names exists is resolveFields.
func readFields(declared []eventFieldFile) []EventField {
	fields := make([]EventField, 0, len(declared))
	for _, field := range declared {
		fields = append(fields, EventField{
			Name: field.Name, Type: field.Type, Mutable: field.Mutable,
		})
	}
	return fields
}

// validateProvides checks the event types this plugin declares it can emit.
//
// Only the shape is checked here. Whether another plugin already provides the
// same type, and whether a subscription names a type nobody provides, are
// questions about a set of bundles rather than about one manifest — the host
// answers them once it has scanned them all.
func validateProvides(manifest Manifest) error {
	records := make(map[string]EventRecord, len(manifest.Records))
	for _, record := range manifest.Records {
		records[record.Name] = record
	}
	provided := make(map[string]struct{}, len(manifest.Provides))
	for _, definition := range manifest.Provides {
		if !validEventType(definition.Type) {
			return fmt.Errorf("plugin %s: invalid provided event type %q, want namespace/name", manifest.ID, definition.Type)
		}
		if _, duplicate := provided[definition.Type]; duplicate {
			return fmt.Errorf("plugin %s: duplicate provided event %s", manifest.ID, definition.Type)
		}
		provided[definition.Type] = struct{}{}
		owner := "event " + definition.Type
		if err := validateFields(manifest.ID, owner, definition.Fields); err != nil {
			return err
		}
		if err := resolveFields(manifest.ID, owner, definition.Fields, records); err != nil {
			return err
		}
	}
	return nil
}

// validEventType requires the namespace/name form.
//
// The slash is what keeps a plugin from providing "block.break" and shadowing a
// native event: no native name has one, so the two vocabularies cannot meet.
// Enforcing it here rather than trusting convention means the collision is
// refused at build time, on the machine that has the source.
func validEventType(eventType string) bool {
	namespace, name, found := strings.Cut(eventType, "/")
	if !found || strings.Contains(name, "/") {
		return false
	}
	return validDottedName(namespace) && validDottedName(name)
}

// validFieldName accepts an identifier as any of the three languages writes it,
// which includes the camelCase a Java record component arrives as.
func validFieldName(name string) bool {
	if name == "" || (name[0] >= '0' && name[0] <= '9') {
		return false
	}
	for _, character := range name {
		switch {
		case character >= 'a' && character <= 'z':
		case character >= 'A' && character <= 'Z':
		case character >= '0' && character <= '9':
		case character == '_':
		default:
			return false
		}
	}
	return true
}

func validBundleReference(reference string) bool {
	if strings.Contains(reference, `\`) || path.IsAbs(reference) {
		return false
	}
	cleaned := path.Clean(reference)
	return cleaned == reference && cleaned != "." && cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}

func validPluginID(id string) bool { return validDottedName(id) }

// validDottedName is the charset a plugin id and both halves of an event type
// share: lowercase, because a name that differs only by case is a name two
// people will spell two ways.
func validDottedName(name string) bool {
	if name == "" || name[0] == '.' || name[len(name)-1] == '.' {
		return false
	}
	for _, character := range name {
		if character != '.' && character != '-' && character != '_' &&
			(character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return false
		}
	}
	return !strings.Contains(name, "..")
}
