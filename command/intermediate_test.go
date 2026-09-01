package command

import (
	"strings"
	"testing"
)

// What gocraft-apt writes for the worked example, spelled the way it spells it.
//
// Captured verbatim from gocraft-apt rather than written from its docs — the
// first draft of this got it wrong, guessing that absent booleans were spelled
// out. This is the place the two languages meet: a change to either side that
// the other would refuse fails here rather than in someone's build.
const aptOutput = `{
  "version": 1,
  "commands": [
    {
      "name": "shop",
      "permission": "shop.use",
      "children": [
        {
          "name": "sell",
          "children": [
            {
              "name": "price",
              "argument": true,
              "kind": "decimal", "min": 0.01, "max": 1000.0,
              "runs": true,
              "children": []
            }
          ]
        },
        {
          "name": "admin",
          "children": [
            {
              "name": "reload",
              "permission": "shop.admin",
              "runs": true,
              "children": []
            }
          ]
        }
      ]
    }
  ]
}`

func TestDecodeIntermediateReadsWhatTheProcessorWrites(t *testing.T) {
	root, err := DecodeIntermediate([]byte(aptOutput))
	if err != nil {
		t.Fatal(err)
	}
	shop := root.Children[0].(Literal)
	if shop.Name != "shop" || shop.Permission != "shop.use" || shop.Exec != 0 {
		t.Fatalf("shop = %+v", shop)
	}
	price := shop.Children[0].(Literal).Children[0].(Argument)
	if price.Type != ArgDecimal || *price.DecimalMin != 0.01 || *price.DecimalMax != 1000 {
		t.Fatalf("price = %+v", price)
	}
	reload := shop.Children[1].(Literal).Children[0].(Literal)
	if reload.Permission != "shop.admin" {
		t.Fatalf("reload = %+v", reload)
	}
	// Minted here, in declaration order, and nowhere else.
	if price.Exec != 1 || reload.Exec != 2 {
		t.Fatalf("executors = %d and %d, want 1 and 2", price.Exec, reload.Exec)
	}
}

// A runtime that builds its tree as code hands over the same file as one that
// extracts it from source. If it did not, "identical for every runtime" would
// be a claim rather than a property.
func TestIntermediateRoundTrips(t *testing.T) {
	minimum, maximum := int64(1), int64(64)
	floor := 0.01
	root := Root{Children: []Node{Literal{
		Name: "shop", Permission: "shop.use", Children: []Node{
			Literal{Name: "sell", Children: []Node{
				Argument{Name: "price", Type: ArgDecimal, DecimalMin: &floor, Exec: 7},
			}},
			Literal{Name: "give", Children: []Node{
				Argument{Name: "player", Type: ArgPlayer, Children: []Node{
					Argument{
						Name: "count", Type: ArgInteger,
						IntegerMin: &minimum, IntegerMax: &maximum, Exec: 9,
					},
				}},
			}},
			Literal{Name: "mode", Children: []Node{
				Argument{Name: "value", Type: ArgEnum, Enum: []string{"buy", "sell"}, Exec: 3},
			}},
		},
	}}}

	encoded, err := EncodeIntermediate(root)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeIntermediate(encoded)
	if err != nil {
		t.Fatalf("%v\n%s", err, encoded)
	}

	shop := decoded.Children[0].(Literal)
	price := shop.Children[0].(Literal).Children[0].(Argument)
	if *price.DecimalMin != 0.01 || price.DecimalMax != nil {
		t.Fatalf("price bounds = %+v", price)
	}
	count := shop.Children[1].(Literal).Children[0].(Argument).Children[0].(Argument)
	if *count.IntegerMin != 1 || *count.IntegerMax != 64 {
		t.Fatalf("count bounds = %+v", count)
	}
	mode := shop.Children[2].(Literal).Children[0].(Argument)
	if len(mode.Enum) != 2 || mode.Enum[1] != "sell" {
		t.Fatalf("mode enum = %+v", mode)
	}
	// The ids the tree went in with are not the ids it comes back with, and
	// that is the point: they are minted by whoever reads this, so a declaring
	// runtime never has an opinion about them.
	if price.Exec != 1 || count.Exec != 2 || mode.Exec != 3 {
		t.Fatalf("executors = %d, %d, %d; want 1, 2, 3", price.Exec, count.Exec, mode.Exec)
	}
}

// A bound left out is absent, not saturated: "open above" and "at most the
// largest long" are different statements, and a renderer shows the second.
func TestEncodeIntermediateOmitsWhatWasNotSaid(t *testing.T) {
	minimum := int64(1)
	root := Root{Children: []Node{Literal{Name: "wait", Children: []Node{
		Argument{Name: "amount", Type: ArgInteger, IntegerMin: &minimum, Exec: 1},
	}}}}
	encoded, err := EncodeIntermediate(root)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if !strings.Contains(text, `"min": 1`) {
		t.Fatalf("no minimum in %s", text)
	}
	if strings.Contains(text, `"max"`) {
		t.Fatalf("saturated maximum in %s", text)
	}
	// Executor ids never appear: they are the reader's to mint.
	if strings.Contains(text, "executor") {
		t.Fatalf("executor id leaked into %s", text)
	}
}

func TestDecodeIntermediateRefusesWhatItCannotHonour(t *testing.T) {
	cases := map[string]string{
		"unknown field":     `{"version":1,"commands":[{"name":"a","runs":true,"colour":"red"}]}`,
		"wrong version":     `{"version":2,"commands":[{"name":"a","runs":true}]}`,
		"nothing declared":  `{"version":1,"commands":[]}`,
		"unknown kind":      `{"version":1,"commands":[{"name":"a","children":[{"name":"b","argument":true,"kind":"colour","runs":true}]}]}`,
		"literal with kind": `{"version":1,"commands":[{"name":"a","kind":"integer","runs":true}]}`,
		"range on a name":   `{"version":1,"commands":[{"name":"a","children":[{"name":"b","argument":true,"kind":"player","min":1,"runs":true}]}]}`,
		"runs nothing":      `{"version":1,"commands":[{"name":"a","children":[{"name":"b"}]}]}`,
	}
	for name, source := range cases {
		if _, err := DecodeIntermediate([]byte(source)); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
}
