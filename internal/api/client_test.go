package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSearchGeneralBuildsSignedRequest(t *testing.T) {
	t.Parallel()
	var requestError string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		for _, key := range []string{"keyword", "q", "search_type", "tab_type", "offset", "limit", "hkey", "_time", "nonce"} {
			if query.Get(key) == "" {
				requestError = "missing query parameter " + key
			}
		}
		if got := request.Header.Get("Cookie"); got != "heybox_id=42" {
			requestError = "unexpected cookie: " + got
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"status":"ok","msg":"","result":{"items":[]}}`))
	}))
	defer server.Close()

	client := NewClient("heybox_id=42", time.Second,
		WithBaseURL(server.URL),
		WithClock(func() time.Time { return time.Unix(1_700_000_000, 0) }),
	)
	_, err := client.SearchGeneral(context.Background(), Query{Keyword: "steam", Type: "post", Offset: 20, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if requestError != "" {
		t.Fatal(requestError)
	}
}

func TestCaptchaIsTyped(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"status":"show_captcha","msg":"","result":{}}`))
	}))
	defer server.Close()

	client := NewClient("", time.Second, WithBaseURL(server.URL))
	_, err := client.SearchGeneral(context.Background(), Query{Keyword: "steam", Type: "all", Limit: 20})
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Kind != ErrorCaptcha {
		t.Fatalf("error = %#v, want captcha error", err)
	}
}

func TestMalformedResponseIsIncompatible(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`not-json`))
	}))
	defer server.Close()

	client := NewClient("", time.Second, WithBaseURL(server.URL))
	_, err := client.SearchTopics(context.Background(), Query{Keyword: "steam", Limit: 20})
	if Kind(err) != ErrorIncompatible {
		t.Fatalf("Kind(error) = %q, want %q", Kind(err), ErrorIncompatible)
	}
}
