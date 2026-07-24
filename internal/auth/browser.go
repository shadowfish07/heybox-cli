package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

const loginEndpoint = "https://login.xiaoheihe.cn/"

type loginResult struct {
	credential Credential
	err        error
}

type LoginSession struct {
	loginURL  string
	state     string
	server    *http.Server
	listener  net.Listener
	result    chan loginResult
	closeOnce sync.Once
}

func NewLoginSession() (*LoginSession, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("启动本地登录回调服务: %w", err)
	}
	state, err := randomState()
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	callbackURL := "http://" + listener.Addr().String() + "/callback"
	loginURL, err := buildLoginURL(callbackURL, state)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	session := &LoginSession{
		loginURL: loginURL,
		state:    state,
		listener: listener,
		result:   make(chan loginResult, 1),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", session.handleCallback)
	mux.HandleFunc("/done", handleDone)
	session.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       5 * time.Second,
		ErrorLog:          log.New(io.Discard, "", 0),
	}
	go func() {
		if serveErr := session.server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			session.deliver(loginResult{err: fmt.Errorf("本地登录回调服务异常: %w", serveErr)})
		}
	}()
	return session, nil
}

func (session *LoginSession) URL() string {
	return session.loginURL
}

func (session *LoginSession) Wait(ctx context.Context) (Credential, error) {
	select {
	case result := <-session.result:
		return result.credential, result.err
	case <-ctx.Done():
		return Credential{}, fmt.Errorf("等待小黑盒登录: %w", ctx.Err())
	}
}

func (session *LoginSession) Close() error {
	var closeErr error
	session.closeOnce.Do(func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		closeErr = session.server.Shutdown(shutdownContext)
		if closeErr != nil {
			_ = session.listener.Close()
		}
	})
	return closeErr
}

func (session *LoginSession) handleCallback(writer http.ResponseWriter, request *http.Request) {
	setCallbackHeaders(writer)
	if request.Method != http.MethodGet {
		http.Error(writer, "只允许 GET 登录回调。", http.StatusMethodNotAllowed)
		return
	}
	query := request.URL.Query()
	if !sameState(query.Get("state"), session.state) {
		http.Error(writer, "登录状态校验失败，请返回终端重新发起登录。", http.StatusBadRequest)
		return
	}
	if providerError := strings.TrimSpace(query.Get("error")); providerError != "" {
		message := strings.TrimSpace(query.Get("error_description"))
		if message == "" {
			message = providerError
		}
		writeResultPage(writer, "登录失败", message, false)
		session.deliver(loginResult{err: fmt.Errorf("小黑盒登录失败: %s", message)})
		return
	}
	credential := Credential{
		HeyboxID:     strings.TrimSpace(query.Get("heybox_id")),
		PKey:         strings.TrimSpace(query.Get("pkey")),
		ExpireAt:     strings.TrimSpace(query.Get("expire_at")),
		XXHHHeyboxID: strings.TrimSpace(query.Get("x_xhh_heyboxid")),
	}
	if err := credential.Validate(); err != nil {
		writeResultPage(writer, "登录回调无效", err.Error(), false)
		session.deliver(loginResult{err: err})
		return
	}
	writeResultPage(writer, "登录成功", "凭据已交给 heybox CLI，可以关闭此页面。", true)
	session.deliver(loginResult{credential: credential})
}

func (session *LoginSession) deliver(result loginResult) {
	select {
	case session.result <- result:
	default:
	}
}

func handleDone(writer http.ResponseWriter, _ *http.Request) {
	setCallbackHeaders(writer)
	writeResultPage(writer, "登录完成", "可以关闭此页面并返回终端。", true)
}

func setCallbackHeaders(writer http.ResponseWriter) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'")
	writer.Header().Set("Referrer-Policy", "no-referrer")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
}

func writeResultPage(writer io.Writer, title, message string, success bool) {
	color := "#b42318"
	if success {
		color = "#16794f"
	}
	_, _ = fmt.Fprintf(writer, `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>%s</title></head><body style="font-family:system-ui,sans-serif;max-width:560px;margin:15vh auto;padding:24px;color:#14191e"><script>history.replaceState(null,"","/done")</script><h1 style="color:%s">%s</h1><p>%s</p></body></html>`, html.EscapeString(title), color, html.EscapeString(title), html.EscapeString(message))
}

func buildLoginURL(callbackURL, state string) (string, error) {
	parsed, err := url.Parse(loginEndpoint)
	if err != nil {
		return "", fmt.Errorf("解析小黑盒登录地址: %w", err)
	}
	query := parsed.Query()
	query.Set("origin", "heybox")
	query.Set("mode", "cli")
	query.Set("state", state)
	query.Set("redirect_url", callbackURL)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func randomState() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("生成登录 state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(random), nil
}

func sameState(actual, expected string) bool {
	if actual == "" || len(actual) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

func OpenURL(rawURL string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", rawURL)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		command = exec.Command("xdg-open", rawURL)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("打开系统浏览器: %w", err)
	}
	go func() { _ = command.Wait() }()
	return nil
}
