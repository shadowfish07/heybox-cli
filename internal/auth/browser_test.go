package auth

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestLoginSessionReceivesValidatedCallback(t *testing.T) {
	t.Parallel()
	session, err := NewLoginSession()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })

	loginURL, err := url.Parse(session.URL())
	if err != nil {
		t.Fatal(err)
	}
	query := loginURL.Query()
	if loginURL.Scheme != "https" || loginURL.Host != "login.xiaoheihe.cn" {
		t.Fatalf("login URL = %q", session.URL())
	}
	if query.Get("mode") != "cli" || query.Get("origin") != "heybox" || len(query.Get("state")) < 40 {
		t.Fatalf("login query = %#v", query)
	}
	callbackURL := query.Get("redirect_url")
	if !strings.HasPrefix(callbackURL, "http://127.0.0.1:") {
		t.Fatalf("callback URL = %q", callbackURL)
	}

	invalidResponse, err := http.Get(callbackURL + "?state=invalid") // #nosec G107 -- local test server URL.
	if err != nil {
		t.Fatal(err)
	}
	_ = invalidResponse.Body.Close()
	if invalidResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid callback status = %d", invalidResponse.StatusCode)
	}

	callbackQuery := url.Values{
		"state":           {query.Get("state")},
		"heybox_id":       {"123456"},
		"pkey":            {"secret-pkey"},
		"expire_at":       {"1999999999"},
		"x_xhh_heyboxid":  {"123456"},
		"google_2fa_pkey": {"must-not-be-stored"},
	}
	response, err := http.Get(callbackURL + "?" + callbackQuery.Encode()) // #nosec G107 -- local test server URL.
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || strings.Contains(string(body), "secret-pkey") {
		t.Fatalf("callback status=%d body=%q", response.StatusCode, body)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	credential, err := session.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if credential.HeyboxID != "123456" || credential.PKey != "secret-pkey" {
		t.Fatalf("credential = %#v", credential)
	}
}

func TestLoginSessionWaitHonorsContext(t *testing.T) {
	t.Parallel()
	session, err := NewLoginSession()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := session.Wait(ctx); err == nil {
		t.Fatal("Wait() error = nil")
	}
}
