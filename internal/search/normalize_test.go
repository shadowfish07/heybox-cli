package search

import (
	"os"
	"testing"
)

func TestNormalizeTopics(t *testing.T) {
	t.Parallel()
	body := fixture(t, "topics.json")
	results := normalizeTopics(body)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Type != "topic" || results[0].ID != "425422" || results[0].Stats.Hot != 61_785_866 {
		t.Fatalf("unexpected first result: %#v", results[0])
	}
}

func TestNormalizeGames(t *testing.T) {
	t.Parallel()
	results := normalizeGames(fixture(t, "games.json"))
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Title != "Steam" || results[0].Stats.Followers != 251_093 {
		t.Fatalf("unexpected result: %#v", results[0])
	}
}

func TestNormalizeGeneral(t *testing.T) {
	t.Parallel()
	results := normalizeGeneral(fixture(t, "general.json"), "all")
	if len(results) != 4 {
		t.Fatalf("len(results) = %d, want 4: %#v", len(results), results)
	}
	if results[0].Type != "post" || results[0].Author != "盒友A" || results[0].Stats.Comments != 16 {
		t.Fatalf("unexpected post: %#v", results[0])
	}
	if results[1].Type != "user" || results[1].Title != "Steam玩家" {
		t.Fatalf("unexpected user: %#v", results[1])
	}
}

func TestNormalizeGeneralFiltersType(t *testing.T) {
	t.Parallel()
	results := normalizeGeneral(fixture(t, "general.json"), "post")
	if len(results) != 1 || results[0].Type != "post" {
		t.Fatalf("results = %#v, want one post", results)
	}
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
