package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.xiaoheihe.cn"
	generalPath    = "/bbs/app/api/general/search/v1"
	topicPath      = "/bbs/app/api/search/topic"
	gamePath       = "/game/search/"
)

type ErrorKind string

const (
	ErrorAuth         ErrorKind = "auth"
	ErrorCaptcha      ErrorKind = "captcha"
	ErrorRateLimit    ErrorKind = "rate_limit"
	ErrorNetwork      ErrorKind = "network"
	ErrorUpstream     ErrorKind = "upstream"
	ErrorIncompatible ErrorKind = "incompatible"
)

type Error struct {
	Kind       ErrorKind
	Message    string
	StatusCode int
	Cause      error
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return string(e.Kind)
}

func (e *Error) Unwrap() error { return e.Cause }

type Query struct {
	Keyword string
	Type    string
	Sort    string
	Offset  int
	Limit   int
}

type Response struct {
	Status string          `json:"status"`
	Msg    string          `json:"msg"`
	Result json.RawMessage `json:"result"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
	cookie     string
	now        func() time.Time
	sleep      func(context.Context, time.Duration) error
}

type ClientOption func(*Client)

func WithBaseURL(baseURL string) ClientOption {
	return func(client *Client) { client.baseURL = strings.TrimRight(baseURL, "/") }
}

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(client *Client) { client.httpClient = httpClient }
}

func WithClock(clock func() time.Time) ClientOption {
	return func(client *Client) { client.now = clock }
}

func NewClient(cookie string, timeout time.Duration, options ...ClientOption) *Client {
	client := &Client{
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		cookie: cookie,
		now:    time.Now,
		sleep: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
	for _, option := range options {
		option(client)
	}
	return client
}

func (c *Client) SearchGeneral(ctx context.Context, query Query) (json.RawMessage, error) {
	params := url.Values{}
	params.Set("keyword", query.Keyword)
	params.Set("q", query.Keyword) // compatibility with older deployments
	params.Set("search_type", generalType(query.Type))
	params.Set("tab_type", generalType(query.Type))
	params.Set("offset", strconv.Itoa(query.Offset))
	params.Set("limit", strconv.Itoa(query.Limit))
	if query.Sort != "" && query.Sort != "relevance" {
		params.Set("sort_filter", query.Sort)
	}
	return c.get(ctx, generalPath, params, true)
}

func (c *Client) SearchTopics(ctx context.Context, query Query) (json.RawMessage, error) {
	params := url.Values{}
	params.Set("q", query.Keyword)
	params.Set("offset", strconv.Itoa(query.Offset))
	params.Set("limit", strconv.Itoa(query.Limit))
	return c.get(ctx, topicPath, params, false)
}

func (c *Client) SearchGames(ctx context.Context, query Query) (json.RawMessage, error) {
	params := url.Values{}
	params.Set("q", query.Keyword)
	params.Set("offset", strconv.Itoa(query.Offset))
	params.Set("limit", strconv.Itoa(query.Limit))
	return c.get(ctx, gamePath, params, false)
}

func generalType(resultType string) string {
	switch resultType {
	case "post":
		return "link"
	case "topic", "user", "game":
		return resultType
	default:
		return "all"
	}
}

func (c *Client) get(ctx context.Context, path string, params url.Values, signed bool) (json.RawMessage, error) {
	if signed {
		params.Set("os_type", "web")
		params.Set("version", "999.0.3")
		params.Set("x_app", "heybox_website")
		params.Set("x_client_type", "web")
		params.Set("x_os_type", webOSType())

		hkey, nonce, timestamp, err := newSignature(path, c.now())
		if err != nil {
			return nil, &Error{Kind: ErrorUpstream, Message: "生成小黑盒请求签名失败", Cause: err}
		}
		params.Set("hkey", hkey)
		params.Set("_time", strconv.FormatInt(timestamp, 10))
		params.Set("nonce", nonce)
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			delay := time.Duration(250*(1<<(attempt-1))+rand.IntN(150)) * time.Millisecond
			if err := c.sleep(ctx, delay); err != nil {
				return nil, &Error{Kind: ErrorNetwork, Message: "请求已取消", Cause: err}
			}
		}

		result, retry, err := c.request(ctx, path, params)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, lastErr
}

func webOSType() string {
	switch runtime.GOOS {
	case "darwin":
		return "Mac"
	case "android":
		return "Android"
	default:
		// The website reports every non-Apple desktop browser as Windows.
		return "Windows"
	}
}

func (c *Client) request(ctx context.Context, path string, params url.Values) (json.RawMessage, bool, error) {
	requestURL := c.baseURL + path + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, false, &Error{Kind: ErrorUpstream, Message: "构造小黑盒请求失败", Cause: err}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "heybox-cli/0.1 (+https://www.xiaoheihe.cn)")
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, true, &Error{Kind: ErrorNetwork, Message: "连接小黑盒失败", Cause: err}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, &Error{Kind: ErrorNetwork, Message: "读取小黑盒响应失败", Cause: err}
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, false, &Error{Kind: ErrorAuth, StatusCode: resp.StatusCode, Message: "小黑盒拒绝了请求；可通过 HEYBOX_COOKIE 提供登录态"}
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, false, &Error{Kind: ErrorRateLimit, StatusCode: resp.StatusCode, Message: "小黑盒请求过于频繁，请稍后再试"}
	}
	if resp.StatusCode >= 500 {
		return nil, true, &Error{Kind: ErrorUpstream, StatusCode: resp.StatusCode, Message: fmt.Sprintf("小黑盒服务暂时不可用（HTTP %d）", resp.StatusCode)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false, &Error{Kind: ErrorUpstream, StatusCode: resp.StatusCode, Message: fmt.Sprintf("小黑盒返回异常状态（HTTP %d）", resp.StatusCode)}
	}

	var envelope Response
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, false, &Error{Kind: ErrorIncompatible, Message: "小黑盒响应格式已变化", Cause: err}
	}
	switch envelope.Status {
	case "ok":
		if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
			return json.RawMessage(`{}`), false, nil
		}
		return envelope.Result, false, nil
	case "show_captcha", "need_captcha":
		return nil, false, &Error{Kind: ErrorCaptcha, Message: "小黑盒要求完成验证码；CLI 不会绕过验证，请稍后重试或设置 HEYBOX_COOKIE"}
	case "lack_token", "need_login", "unauthorized":
		return nil, false, &Error{Kind: ErrorAuth, Message: "搜索需要小黑盒登录态；请设置 HEYBOX_COOKIE"}
	case "failed":
		message := strings.TrimSpace(envelope.Msg)
		if message == "" {
			message = "小黑盒搜索请求失败"
		}
		if strings.Contains(message, "频繁") || strings.Contains(message, "限流") {
			return nil, false, &Error{Kind: ErrorRateLimit, Message: message}
		}
		return nil, false, &Error{Kind: ErrorUpstream, Message: message}
	default:
		return nil, false, &Error{Kind: ErrorIncompatible, Message: fmt.Sprintf("无法识别的小黑盒响应状态：%s", envelope.Status)}
	}
}

func Kind(err error) ErrorKind {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr.Kind
	}
	return ErrorUpstream
}
