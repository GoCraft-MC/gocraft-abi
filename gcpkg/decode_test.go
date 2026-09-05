package gcpkg

import (
	"strings"
	"testing"
)

const minimalManifest = `id = "fr.oreo.hello"
version = "0.1.0"
api = 1
runtime = "lua"
`

func TestDecodeManifestNamesUnknownFieldsWithTheirLine(t *testing.T) {
	_, err := DecodeManifest(strings.NewReader(minimalManifest + `description = "nope"` + "\n"))
	if err == nil {
		t.Fatal("DecodeManifest() accepted an unknown field")
	}
	if want := `plugin.toml:5: unknown field "description"`; err.Error() != want {
		t.Fatalf("DecodeManifest() error = %q, want %q", err.Error(), want)
	}
}

func TestDecodeManifestReportsSyntaxErrorsWithAPosition(t *testing.T) {
	_, err := DecodeManifest(strings.NewReader("id = \"fr.oreo.hello\"\nversion =\n"))
	if err == nil {
		t.Fatal("DecodeManifest() accepted malformed TOML")
	}
	if !strings.HasPrefix(err.Error(), ManifestFileName+":") {
		t.Fatalf("DecodeManifest() error = %v, want a %s:line:column prefix", err, ManifestFileName)
	}
}

func TestDecodeManifestAcceptsAMinimalManifest(t *testing.T) {
	manifest, err := DecodeManifest(strings.NewReader(minimalManifest))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "fr.oreo.hello" || manifest.Runtime != "lua" || manifest.APIVersion != CurrentAPIVersion {
		t.Fatalf("DecodeManifest() = %+v", manifest)
	}
	if len(manifest.Subscriptions) != 0 || len(manifest.Permissions) != 0 {
		t.Fatalf("DecodeManifest() subscriptions = %+v, permissions = %+v", manifest.Subscriptions, manifest.Permissions)
	}
	if len(manifest.Provides) != 0 {
		t.Fatalf("DecodeManifest() provides = %+v, want none", manifest.Provides)
	}
}

const purchaseEvent = `
[[events.provides]]
type = "fr.oreo.shop/purchase"
cancellable = true
fail_closed = true
fields = [
  { name = "player", type = "PlayerRef", mutable = false },
  { name = "tiers", type = "[]fr.oreo.Tier", mutable = false },
  { name = "price", type = "double", mutable = true },
]
`

func TestDecodeManifestReadsAProvidedEvent(t *testing.T) {
	manifest, err := DecodeManifest(strings.NewReader(minimalManifest + purchaseEvent))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Provides) != 1 {
		t.Fatalf("DecodeManifest() provides = %+v, want one event", manifest.Provides)
	}
	definition := manifest.Provides[0]
	if definition.Type != "fr.oreo.shop/purchase" || !definition.Cancellable || !definition.FailClosed {
		t.Fatalf("DecodeManifest() event = %+v", definition)
	}
	want := []EventField{
		{Name: "player", Type: "PlayerRef", Mutable: false},
		{Name: "tiers", Type: "[]fr.oreo.Tier", Mutable: false},
		{Name: "price", Type: "double", Mutable: true},
	}
	if len(definition.Fields) != len(want) {
		t.Fatalf("DecodeManifest() fields = %+v, want %+v", definition.Fields, want)
	}
	for index, field := range definition.Fields {
		if field != want[index] {
			t.Fatalf("DecodeManifest() field %d = %+v, want %+v", index, field, want[index])
		}
	}
}

func TestDecodeManifestRejectsProvidedEvents(t *testing.T) {
	cases := []struct {
		name     string
		manifest string
		want     string
	}{{
		name:     "a type with no namespace shadows a native event",
		manifest: "[[events.provides]]\ntype = \"block.break\"\n",
		want:     "invalid provided event type",
	}, {
		name:     "an empty namespace",
		manifest: "[[events.provides]]\ntype = \"/purchase\"\n",
		want:     "invalid provided event type",
	}, {
		name:     "an empty name",
		manifest: "[[events.provides]]\ntype = \"fr.oreo.shop/\"\n",
		want:     "invalid provided event type",
	}, {
		name:     "an uppercase namespace",
		manifest: "[[events.provides]]\ntype = \"fr.Oreo/purchase\"\n",
		want:     "invalid provided event type",
	}, {
		name:     "the same event twice",
		manifest: "[[events.provides]]\ntype = \"fr.oreo/a\"\n[[events.provides]]\ntype = \"fr.oreo/a\"\n",
		want:     "duplicate provided event",
	}, {
		name:     "two fields with one name",
		manifest: "[[events.provides]]\ntype = \"fr.oreo/a\"\nfields = [{ name = \"x\", type = \"double\" }, { name = \"x\", type = \"double\" }]\n",
		want:     "duplicate field",
	}, {
		name:     "a field with no type",
		manifest: "[[events.provides]]\ntype = \"fr.oreo/a\"\nfields = [{ name = \"x\", type = \"\" }]\n",
		want:     "has no type",
	}, {
		name:     "a field name that is not an identifier",
		manifest: "[[events.provides]]\ntype = \"fr.oreo/a\"\nfields = [{ name = \"a price\", type = \"double\" }]\n",
		want:     "invalid field name",
	}}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeManifest(strings.NewReader(minimalManifest + test.manifest))
			if err == nil {
				t.Fatalf("DecodeManifest() accepted %s", test.name)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("DecodeManifest() error = %v, want it to mention %q", err, test.want)
			}
		})
	}
}

func TestDecodeManifestAcceptsAnEventWithNoFields(t *testing.T) {
	manifest, err := DecodeManifest(strings.NewReader(minimalManifest +
		"[[events.provides]]\ntype = \"fr.oreo.shop/opened\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Provides) != 1 || len(manifest.Provides[0].Fields) != 0 {
		t.Fatalf("DecodeManifest() provides = %+v, want one event carrying nothing", manifest.Provides)
	}
	if manifest.Provides[0].Cancellable || manifest.Provides[0].FailClosed {
		t.Fatalf("DecodeManifest() event = %+v, want both flags off by default", manifest.Provides[0])
	}
}
