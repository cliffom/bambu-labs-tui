// Package cloud implements the read-only account API and login challenges.
package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/cliffom/bambu-labs-tui/internal/farm"
)

var (
	ErrAuth        = errors.New("Bambu session expired or rejected; run bambu-tui login")
	ErrBlocked     = errors.New("Bambu Cloud rejected the request; try again later or import an existing token with bambu-tui login --token")
	ErrRateLimited = errors.New("Bambu Cloud rate limit reached; retry later")
)

type Client struct {
	HTTP                          *http.Client
	BaseURL, SiteURL, MQTTAddress string
}
type Session struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}
type LoginResult struct {
	AccessToken string `json:"accessToken"`
	LoginType   string `json:"loginType"`
	TFAKey      string `json:"tfaKey"`
}

func New(region string) *Client {
	jar, _ := cookiejar.New(nil)
	c := &Client{HTTP: &http.Client{Timeout: 15 * time.Second, Jar: jar}, BaseURL: "https://api.bambulab.com", SiteURL: "https://bambulab.com", MQTTAddress: "us.mqtt.bambulab.com:8883"}
	if region == "china" {
		c.BaseURL = "https://api.bambulab.cn"
		c.SiteURL = "https://bambulab.cn"
		c.MQTTAddress = "cn.mqtt.bambulab.com:8883"
	}
	// Do not forward account credentials or tokens through unexpected redirects.
	c.HTTP.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return c
}

func (c *Client) request(ctx context.Context, method, url, token string, body any, extra http.Header) ([]byte, *http.Response, error) {
	var input io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, nil, errors.New("encode account request")
		}
		input = bytes.NewReader(b)
	}
	r, err := http.NewRequestWithContext(ctx, method, url, input)
	if err != nil {
		return nil, nil, errors.New("invalid account endpoint")
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	r.Header.Set("User-Agent", "bambu_network_agent/01.09.05.01")
	r.Header.Set("X-BBL-Client-Name", "OrcaSlicer")
	r.Header.Set("X-BBL-Client-Type", "slicer")
	r.Header.Set("X-BBL-Client-Version", "01.09.05.51")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	for k, values := range extra {
		for _, v := range values {
			r.Header.Add(k, v)
		}
	}
	resp, err := c.HTTP.Do(r)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, errors.New("cannot reach Bambu Cloud")
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024+1))
	if err != nil || len(data) > 2*1024*1024 {
		return nil, resp, errors.New("invalid account response")
	}
	switch resp.StatusCode {
	case 401:
		return nil, resp, ErrAuth
	case 403:
		return nil, resp, ErrBlocked
	case 429:
		return nil, resp, ErrRateLimited
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp, fmt.Errorf("Bambu Cloud request failed (HTTP %d)", resp.StatusCode)
	}
	// Never expose a response body: it may contain account secrets.
	return data, resp, nil
}

func decode(data []byte, dest any) error {
	if err := json.Unmarshal(data, dest); err != nil {
		return errors.New("unrecognized Bambu Cloud response")
	}
	var status struct {
		Code  json.RawMessage `json:"code"`
		Error json.RawMessage `json:"error"`
	}
	_ = json.Unmarshal(data, &status)
	if len(status.Code) > 0 && string(status.Code) != "null" && string(status.Code) != "0" && string(status.Code) != `"0"` {
		return errors.New("Bambu Cloud rejected the request or verification code")
	}
	if len(status.Error) > 0 && string(status.Error) != "null" && string(status.Error) != `""` {
		return errors.New("Bambu Cloud returned an account error")
	}
	return nil
}

func (c *Client) Login(ctx context.Context, email, password, code string) (LoginResult, error) {
	input := map[string]string{"account": email}
	if code != "" {
		input["code"] = code
	} else {
		input["password"] = password
	}
	data, _, err := c.request(ctx, "POST", c.BaseURL+"/v1/user-service/user/login", "", input, nil)
	if err != nil {
		return LoginResult{}, err
	}
	var r LoginResult
	if err := decode(data, &r); err != nil {
		return r, err
	}
	if r.AccessToken == "" && r.LoginType != "verifyCode" && r.LoginType != "tfa" {
		return r, errors.New("unsupported login response; try token import")
	}
	return r, nil
}
func (c *Client) SendCode(ctx context.Context, email string) error {
	data, _, err := c.request(ctx, "POST", c.BaseURL+"/v1/user-service/user/sendemail/code", "", map[string]string{"email": email, "type": "codeLogin"}, nil)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	var r map[string]any
	return decode(data, &r)
}
func (c *Client) TwoFactor(ctx context.Context, key, code string) (string, error) {
	_, resp, err := c.request(ctx, "GET", c.SiteURL+"/api/csrf", "", nil, nil)
	if err != nil {
		return "", err
	}
	csrf := ""
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "bbl_csrf_token" {
			csrf = cookie.Value
		}
	}
	if csrf == "" {
		return "", errors.New("Bambu did not provide a sign-in cookie; try token import")
	}
	data, resp, err := c.request(ctx, "POST", c.SiteURL+"/api/sign-in/tfa", "", map[string]string{"tfaKey": key, "tfaCode": code}, http.Header{"X-Bbl-Csrf-Token": {csrf}})
	if err != nil {
		return "", err
	}
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "token" && cookie.Value != "" {
			return cookie.Value, nil
		}
	}
	var r LoginResult
	if err := decode(data, &r); err != nil {
		return "", err
	}
	if r.AccessToken == "" {
		return "", errors.New("authenticator code rejected; try login again")
	}
	return r.AccessToken, nil
}
func (c *Client) Session(ctx context.Context, token string) (Session, error) {
	data, _, err := c.request(ctx, "GET", c.BaseURL+"/v1/design-user-service/my/preference", token, nil, nil)
	if err != nil {
		return Session{}, err
	}
	var r struct {
		UID json.RawMessage `json:"uid"`
	}
	if err := decode(data, &r); err != nil {
		return Session{}, err
	}
	uid := strings.Trim(string(r.UID), `"`)
	if uid == "" || uid == "null" {
		return Session{}, errors.New("account response has no MQTT user ID")
	}
	for _, ch := range uid {
		if ch < '0' || ch > '9' {
			return Session{}, errors.New("account response has invalid MQTT user ID")
		}
	}
	return Session{token, "u_" + uid}, nil
}
func (c *Client) Devices(ctx context.Context, token string) ([]farm.Device, error) {
	data, _, err := c.request(ctx, "GET", c.BaseURL+"/v1/iot-service/api/user/bind", token, nil, nil)
	if err != nil {
		return nil, err
	}
	var r struct {
		Devices []farm.Device `json:"devices"`
	}
	if err := decode(data, &r); err != nil {
		return nil, err
	}
	for i := range r.Devices {
		if r.Devices[i].Model == "" {
			r.Devices[i].Model = r.Devices[i].ModelCode
		}
	}
	return r.Devices, nil
}
