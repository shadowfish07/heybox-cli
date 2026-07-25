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

func TestGeneralTypeUsesLinkForAll(t *testing.T) {
	t.Parallel()
	if got := generalType("all"); got != "link" {
		t.Fatalf("generalType(all) = %q, want link", got)
	}
}

func TestQRLoginFlow(t *testing.T) {
	t.Parallel()
	polls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		for _, key := range []string{"hkey", "_time", "nonce", "app"} {
			if request.URL.Query().Get(key) == "" {
				t.Errorf("%s: missing query parameter %s", request.URL.Path, key)
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case qrCreatePath:
			_, _ = writer.Write([]byte(`{"status":"ok","result":{"qr_url":"https://api.xiaoheihe.cn/account/qr_login/?qr=token-1","expire":120}}`))
		case qrStatePath:
			if request.URL.Query().Get("qr") != "token-1" {
				t.Errorf("qr token = %q", request.URL.Query().Get("qr"))
			}
			polls++
			if polls == 1 {
				_, _ = writer.Write([]byte(`{"status":"ok","result":{"error":"ready"}}`))
				return
			}
			http.SetCookie(writer, &http.Cookie{Name: "pkey", Value: "cookie-secret", Path: "/"})
			_, _ = writer.Write([]byte(`{"status":"ok","result":{"error":"ok","expire_at":1999999999,"profile":{"heybox_id":42}}}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := NewClient("", time.Second, WithBaseURL(server.URL))
	challenge, err := client.CreateQRLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if challenge.Token != "token-1" || challenge.ExpiresIn != 120*time.Second {
		t.Fatalf("challenge = %#v", challenge)
	}
	first, err := client.PollQRLogin(context.Background(), challenge.Token)
	if err != nil || first.State != QRLoginScanned {
		t.Fatalf("first poll = %#v, %v", first, err)
	}
	second, err := client.PollQRLogin(context.Background(), challenge.Token)
	if err != nil || second.State != QRLoginSucceeded || second.HeyboxID != "42" || second.PKey != "cookie-secret" || second.ExpireAt != "1999999999" {
		t.Fatalf("second poll = %#v, %v", second, err)
	}
}

func TestQRLoginRequiresBrowserForTwoFactor(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"status":"need_google_check","msg":"verify","result":{}}`))
	}))
	defer server.Close()
	client := NewClient("", time.Second, WithBaseURL(server.URL))
	_, err := client.PollQRLogin(context.Background(), "token")
	if Kind(err) != ErrorAuth {
		t.Fatalf("error = %v", err)
	}
}
