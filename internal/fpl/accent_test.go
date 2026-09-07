package fpl

import (
	"encoding/json"
	"testing"
)

func TestStripAccents(t *testing.T) {
	cases := map[string]string{
		"Højlund":     "Hojlund",
		"Muñoz":       "Munoz",
		"Sánchez":     "Sanchez",
		"Højbjerg":    "Hojbjerg",
		"Håland":      "Haland",
		"Lewandowski": "Lewandowski",
		"Cucurella":   "Cucurella",
		"O'Riley":     "O'Riley",
		"Aït-Nouri":   "Ait-Nouri",
		"Große":       "Grosse",
		"Skjær":       "Skjaer",
		"Łukasz":      "Lukasz",
		"":            "",
		"plain ascii": "plain ascii",
	}
	for in, want := range cases {
		if got := stripAccents(in); got != want {
			t.Errorf("stripAccents(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNumUnmarshal(t *testing.T) {
	type holder struct {
		V num `json:"v"`
	}
	cases := map[string]float64{
		`{"v":"4.0"}`:  4.0,
		`{"v":4.0}`:    4.0,
		`{"v":"0.00"}`: 0,
		`{"v":null}`:   0,
		`{"v":""}`:     0,
		`{"v":"12"}`:   12,
		`{"v":-1.5}`:   -1.5,
	}
	for in, want := range cases {
		var h holder
		if err := json.Unmarshal([]byte(in), &h); err != nil {
			t.Errorf("unmarshal %s: %v", in, err)
			continue
		}
		if h.V.Float() != want {
			t.Errorf("num from %s = %v, want %v", in, h.V.Float(), want)
		}
	}
}
