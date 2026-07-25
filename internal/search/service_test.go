package search

import (
	"context"
	"errors"
	"testing"
)

type fakeAPI struct {
	general        []byte
	topics         []byte
	games          []byte
	err            error
	calls          int
	generalQueries []APIQuery
}

func (f *fakeAPI) SearchGeneral(_ context.Context, query APIQuery) ([]byte, error) {
	f.calls++
	f.generalQueries = append(f.generalQueries, query)
	return f.general, f.err
}

func (f *fakeAPI) SearchTopics(_ context.Context, _ APIQuery) ([]byte, error) {
	return f.topics, nil
}

func (f *fakeAPI) SearchGames(_ context.Context, _ APIQuery) ([]byte, error) {
	return f.games, nil
}

func TestAllFallsBackWithPartialWarning(t *testing.T) {
	t.Parallel()
	upstream := &fakeAPI{
		topics: fixture(t, "topics.json"),
		games:  fixture(t, "games.json"),
		err:    errors.New("captcha"),
	}
	service := NewService(upstream)
	page, err := service.Search(context.Background(), Options{Keyword: "steam", Type: "all", Sort: "relevance", Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if !page.Partial || len(page.Warnings) == 0 || len(page.Results) != 3 {
		t.Fatalf("unexpected page: %#v", page)
	}
}

func TestAllPaginatesAndDeduplicates(t *testing.T) {
	t.Parallel()
	upstream := &fakeAPI{
		general: fixture(t, "general.json"),
		topics:  fixture(t, "topics.json"),
		games:   fixture(t, "games.json"),
	}
	service := NewService(upstream)
	service.pageWait = 0
	page, err := service.Search(context.Background(), Options{Keyword: "steam", Type: "all", Page: 1, Limit: 4, All: true, MaxPages: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Results) != 4 || upstream.calls != 4 {
		t.Fatalf("results=%d calls=%d, want 4 results and 4 general calls", len(page.Results), upstream.calls)
	}
	if upstream.generalQueries[0].Type != "post" || upstream.generalQueries[1].Type != "user" {
		t.Fatalf("general query types = %q, %q", upstream.generalQueries[0].Type, upstream.generalQueries[1].Type)
	}
}

func TestAllAggregatesEachSearchType(t *testing.T) {
	t.Parallel()
	upstream := &fakeAPI{
		general: fixture(t, "general.json"),
		topics:  fixture(t, "topics.json"),
		games:   fixture(t, "games.json"),
	}
	service := NewService(upstream)
	page, err := service.Search(context.Background(), Options{Keyword: "steam", Type: "all", Page: 1, Limit: 4})
	if err != nil {
		t.Fatal(err)
	}
	if page.Partial || len(page.Warnings) != 0 || len(page.Results) != 4 {
		t.Fatalf("unexpected page: %#v", page)
	}
	wantTypes := []string{"post", "topic", "user", "game"}
	for index, wantType := range wantTypes {
		if page.Results[index].Type != wantType {
			t.Fatalf("result %d type = %q, want %q", index, page.Results[index].Type, wantType)
		}
	}
}

func TestInterleaveRespectsLimit(t *testing.T) {
	t.Parallel()
	topics := []Result{{Type: "topic", ID: "1"}, {Type: "topic", ID: "2"}, {Type: "topic", ID: "3"}}
	games := []Result{{Type: "game", ID: "1"}, {Type: "game", ID: "2"}}
	results := interleave(3, topics, games)
	if len(results) != 3 || results[0].Type != "topic" || results[1].Type != "game" || results[2].ID != "2" {
		t.Fatalf("unexpected interleaved results: %#v", results)
	}
}
