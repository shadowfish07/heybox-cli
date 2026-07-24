package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var ErrNotLoggedIn = errors.New("尚未登录小黑盒")

type Credential struct {
	HeyboxID     string `json:"heybox_id"`
	PKey         string `json:"pkey"`
	ExpireAt     string `json:"expire_at,omitempty"`
	XXHHHeyboxID string `json:"x_xhh_heyboxid,omitempty"`
}

func (credential Credential) Validate() error {
	if strings.TrimSpace(credential.HeyboxID) == "" {
		return fmt.Errorf("登录回调缺少 heybox_id")
	}
	if strings.TrimSpace(credential.PKey) == "" {
		return fmt.Errorf("登录回调缺少 pkey")
	}
	return nil
}

func (credential Credential) Cookie() string {
	xhhID := strings.TrimSpace(credential.XXHHHeyboxID)
	if xhhID == "" {
		xhhID = strings.TrimSpace(credential.HeyboxID)
	}
	values := [][2]string{
		{"heybox_id", credential.HeyboxID},
		{"user_heybox_id", credential.HeyboxID},
		{"pkey", credential.PKey},
		{"user_pkey", credential.PKey},
		{"x_xhh_heyboxid", xhhID},
	}
	cookies := make([]string, 0, len(values))
	for _, value := range values {
		cookie := (&http.Cookie{Name: value[0], Value: value[1]}).String()
		if cookie != "" {
			cookies = append(cookies, cookie)
		}
	}
	return strings.Join(cookies, "; ")
}
