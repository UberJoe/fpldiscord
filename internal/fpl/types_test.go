package fpl

import (
	"encoding/json"
	"testing"
)

// The Draft /event/{gw}/live feed serialises each explain entry as a
// [statsArray, fixtureId] tuple, not the {"fixture": N} object the main site
// uses. LiveExplain must read the trailing fixture id and ignore the stats half.
func TestLiveExplain_DecodesDraftTuple(t *testing.T) {
	const raw = `{
	  "stats": {"minutes": 0, "total_points": 0},
	  "explain": [
	    [[{"name": "Minutes played", "points": 0, "value": 0, "stat": "minutes"}], 29]
	  ]
	}`

	var le LiveElement
	if err := json.Unmarshal([]byte(raw), &le); err != nil {
		t.Fatalf("unmarshal live element: %v", err)
	}
	if len(le.Explain) != 1 {
		t.Fatalf("len(Explain) = %d, want 1", len(le.Explain))
	}
	if le.Explain[0].Fixture != 29 {
		t.Errorf("Explain[0].Fixture = %d, want 29", le.Explain[0].Fixture)
	}
}

// A double gameweek links two fixtures — one tuple per fixture.
func TestLiveExplain_DecodesDoubleGameweekTuples(t *testing.T) {
	const raw = `{
	  "stats": {"minutes": 90, "total_points": 8},
	  "explain": [
	    [[{"stat": "minutes", "points": 2, "value": 90}], 12],
	    [[{"stat": "goals_scored", "points": 4, "value": 1}], 37]
	  ]
	}`

	var le LiveElement
	if err := json.Unmarshal([]byte(raw), &le); err != nil {
		t.Fatalf("unmarshal live element: %v", err)
	}
	got := []int{}
	for _, ex := range le.Explain {
		got = append(got, ex.Fixture)
	}
	if len(got) != 2 || got[0] != 12 || got[1] != 37 {
		t.Errorf("fixture ids = %v, want [12 37]", got)
	}
}

// An empty or null explain decodes to an empty slice — a starter with no linked
// fixture yet (kickoff still to come) must not error the whole snapshot.
func TestLiveExplain_EmptyAndNullDecodeToNoFixtures(t *testing.T) {
	for name, raw := range map[string]string{
		"empty":  `{"stats": {"minutes": 0}, "explain": []}`,
		"null":   `{"stats": {"minutes": 0}, "explain": null}`,
		"absent": `{"stats": {"minutes": 0}}`,
	} {
		t.Run(name, func(t *testing.T) {
			var le LiveElement
			if err := json.Unmarshal([]byte(raw), &le); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(le.Explain) != 0 {
				t.Errorf("len(Explain) = %d, want 0", len(le.Explain))
			}
		})
	}
}

// The tuple shape must survive a full LiveGW decode (object-keyed elements go
// through LiveGW.UnmarshalJSON), so the auto-sub fixture join still works.
func TestLiveGW_DecodesExplainTupleThroughElementMap(t *testing.T) {
	const raw = `{
	  "elements": {
	    "105": {"stats": {"minutes": 0, "total_points": 0},
	            "explain": [[[{"stat": "minutes", "points": 0, "value": 0}], 1]]}
	  },
	  "fixtures": [
	    {"id": 1, "started": true, "finished": true, "finished_provisional": true}
	  ]
	}`

	var gw LiveGW
	if err := json.Unmarshal([]byte(raw), &gw); err != nil {
		t.Fatalf("unmarshal live gw: %v", err)
	}
	ex := gw.Elements[ElementID(105)].Explain
	if len(ex) != 1 || ex[0].Fixture != 1 {
		t.Fatalf("element 105 explain = %+v, want one entry linking fixture 1", ex)
	}
}
