package gcpkg

import (
	"strings"
	"testing"
)

func TestParseFieldTypeReadsTheVocabulary(t *testing.T) {
	cases := map[string]FieldType{
		"double":         {Element: "double"},
		"bytes":          {Element: "bytes"},
		"PlayerRef":      {Element: "PlayerRef"},
		"fr.oreo.Tier":   {Element: "fr.oreo.Tier", Record: true},
		"[]double":       {List: true, Element: "double"},
		"[]fr.oreo.Tier": {List: true, Element: "fr.oreo.Tier", Record: true},
	}
	for declared, want := range cases {
		got, ok := ParseFieldType(declared)
		if !ok || got != want {
			t.Fatalf("ParseFieldType(%q) = %+v, %v, want %+v", declared, got, ok, want)
		}
	}
	for _, declared := range []string{
		"", "   ", "[]", "[][]double", "Map<String,Integer>",
		// An event type, not a field type: the two vocabularies are separate on
		// purpose, and the slash is what tells them apart.
		"fr.oreo.shop/purchase",
		".Tier", "Tier.", "fr..oreo.Tier", "1Tier",
	} {
		if _, ok := ParseFieldType(declared); ok {
			t.Fatalf("ParseFieldType(%q) accepted it", declared)
		}
	}
}

// A record is a type, and every language this contract serves capitalises one.
// Refusing Tier would mean making an author spell their own class differently
// here than in their source.
func TestRecordNamesKeepTheirCase(t *testing.T) {
	manifest := minimalManifest + `
[[events.types]]
name = "fr.oreo.Tier"
fields = [{ name = "label", type = "string" }]
`
	if _, err := DecodeManifest(strings.NewReader(manifest)); err != nil {
		t.Fatalf("DecodeManifest() refused a capitalised record: %v", err)
	}
}

func TestDecodeManifestRejectsRecords(t *testing.T) {
	cases := []struct {
		name     string
		manifest string
		want     string
	}{{
		name: "a field naming a record nothing declares",
		manifest: "[[events.provides]]\ntype = \"fr.oreo/a\"\n" +
			"fields = [{ name = \"tiers\", type = \"[]fr.oreo.Tier\" }]\n",
		want: "which no [[events.types]] declares",
	}, {
		name: "a record naming a record nothing declares",
		manifest: "[[events.types]]\nname = \"fr.oreo.Basket\"\n" +
			"fields = [{ name = \"tiers\", type = \"[]fr.oreo.Tier\" }]\n",
		want: "which no [[events.types]] declares",
	}, {
		name: "the same record twice",
		manifest: "[[events.types]]\nname = \"fr.oreo.Tier\"\nfields = [{ name = \"a\", type = \"int\" }]\n" +
			"[[events.types]]\nname = \"fr.oreo.Tier\"\nfields = [{ name = \"a\", type = \"int\" }]\n",
		want: "duplicate record",
	}, {
		name:     "a record carrying nothing",
		manifest: "[[events.types]]\nname = \"fr.oreo.Tier\"\n",
		want:     "carries no fields",
	}, {
		name:     "a record named like an event type",
		manifest: "[[events.types]]\nname = \"fr.oreo/Tier\"\nfields = [{ name = \"a\", type = \"int\" }]\n",
		want:     "invalid record name",
	}, {
		// The wire is a finite positional payload with no pointers, so this is
		// not a shape that could be encoded at all.
		name: "a record containing itself",
		manifest: "[[events.types]]\nname = \"fr.oreo.Node\"\n" +
			"fields = [{ name = \"child\", type = \"fr.oreo.Node\" }]\n",
		want: "contains itself",
	}, {
		name: "a record containing itself through another",
		manifest: "[[events.types]]\nname = \"fr.oreo.A\"\nfields = [{ name = \"b\", type = \"fr.oreo.B\" }]\n" +
			"[[events.types]]\nname = \"fr.oreo.B\"\nfields = [{ name = \"a\", type = \"[]fr.oreo.A\" }]\n",
		want: "contains itself",
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

// Declared once and referenced by name, so a record two events carry is
// described in one place — and two plugins comparing their manifests compare
// one description rather than copies that happen to agree.
func TestDecodeManifestReadsARecordSharedByTwoEvents(t *testing.T) {
	manifest, err := DecodeManifest(strings.NewReader(minimalManifest + `
[[events.types]]
name = "fr.oreo.Tier"
fields = [
  { name = "label", type = "string" },
  { name = "price", type = "double", mutable = true },
]

[[events.provides]]
type = "fr.oreo.shop/purchase"
fields = [{ name = "tiers", type = "[]fr.oreo.Tier" }]

[[events.provides]]
type = "fr.oreo.shop/refund"
fields = [{ name = "tier", type = "fr.oreo.Tier" }]
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Records) != 1 || manifest.Records[0].Name != "fr.oreo.Tier" {
		t.Fatalf("DecodeManifest() records = %+v", manifest.Records)
	}
	if len(manifest.Records[0].Fields) != 2 || !manifest.Records[0].Fields[1].Mutable {
		t.Fatalf("DecodeManifest() record fields = %+v", manifest.Records[0].Fields)
	}
	if len(manifest.Provides) != 2 {
		t.Fatalf("DecodeManifest() provides = %+v, want two events", manifest.Provides)
	}
}
