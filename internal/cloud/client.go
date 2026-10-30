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

	"github.com/cliffom/bambustand/internal/farm"
)

var (
	ErrAuth        = errors.New("account session expired or rejected; run bambustand login")
	ErrBlocked     = errors.New("request rejected by Bambu Cloud; try again later or import an existing token with bambustand login --token")
	ErrRateLimited = errors.New("rate limited by Bambu Cloud; retry later")
)

type (
	loginRequest struct {
		Account  string  `json:"account"`
		Code     *string `json:"code,omitempty"`
		Password *string `json:"password,omitempty"`
	}
	emailCodeRequest struct {
		Email string `json:"email"`
		Type  string `json:"type"`
	}
	twoFactorRequest struct {
		Key  string `json:"tfaKey"`
		Code string `json:"tfaCode"`
	}

	Client struct {
		HTTP                          *http.Client
		BaseURL, SiteURL, MQTTAddress string
	}

	Session struct {
		Token    string `json:"token"`
		Username string `json:"username"`
	}

	LoginResult struct {
		AccessToken string `json:"accessToken"`
		LoginType   string `json:"loginType"`
		TFAKey      string `json:"tfaKey"`
	}
)

func New(region string) *Client {
	jar, _ := cookiejar.New(nil)
	addresses := endpointsFor(region)
	c := &Client{HTTP: &http.Client{Timeout: requestTimeout, Jar: jar}, BaseURL: addresses.api, SiteURL: addresses.site, MQTTAddress: addresses.mqtt}
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
	r.Header.Set("Content-Type", jsonContentType)
	r.Header.Set("Accept", jsonContentType)
	r.Header.Set("User-Agent", userAgent)
	r.Header.Set("X-BBL-Client-Name", clientName)
	r.Header.Set("X-BBL-Client-Type", clientType)
	r.Header.Set("X-BBL-Client-Version", clientVersion)
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
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil || len(data) > maxResponseBytes {
		return nil, resp, errors.New("invalid account response")
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return nil, resp, ErrAuth
	case http.StatusForbidden:
		return nil, resp, ErrBlocked
	case http.StatusTooManyRequests:
		return nil, resp, ErrRateLimited
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, resp, fmt.Errorf("cloud request failed (HTTP %d)", resp.StatusCode)
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
		return errors.New("cloud request or verification code rejected")
	}
	if len(status.Error) > 0 && string(status.Error) != "null" && string(status.Error) != `""` {
		return errors.New("cloud account error returned")
	}
	return nil
}

func (c *Client) Login(ctx context.Context, email, password, code string) (LoginResult, error) {
	input := loginRequest{Account: email}
	if code != "" {
		input.Code = &code
	} else {
		input.Password = &password
	}
	data, _, err := c.request(ctx, http.MethodPost, c.BaseURL+loginPath, "", input, nil)
	if err != nil {
		return LoginResult{}, err
	}
	var r LoginResult
	if err := decode(data, &r); err != nil {
		return r, err
	}
	if r.AccessToken == "" && r.LoginType != LoginVerifyCode && r.LoginType != LoginTFA {
		return r, errors.New("unsupported login response; try token import")
	}
	return r, nil
}

func (c *Client) SendCode(ctx context.Context, email string) error {
	data, _, err := c.request(ctx, http.MethodPost, c.BaseURL+sendCodePath, "", emailCodeRequest{Email: email, Type: emailCodeLogin}, nil)
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
	_, resp, err := c.request(ctx, http.MethodGet, c.SiteURL+csrfPath, "", nil, nil)
	if err != nil {
		return "", err
	}
	csrf := ""
	for _, cookie := range resp.Cookies() {
		if cookie.Name == csrfCookieName {
			csrf = cookie.Value
		}
	}
	if csrf == "" {
		return "", errors.New("no sign-in cookie provided by Bambu; try token import")
	}
	data, resp, err := c.request(ctx, http.MethodPost, c.SiteURL+twoFactorPath, "", twoFactorRequest{Key: key, Code: code}, http.Header{csrfHeader: {csrf}})
	if err != nil {
		return "", err
	}
	for _, cookie := range resp.Cookies() {
		if cookie.Name == tokenCookieName && cookie.Value != "" {
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
	data, _, err := c.request(ctx, http.MethodGet, c.BaseURL+preferencesPath, token, nil, nil)
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
	return Session{token, mqttUsernamePrefix + uid}, nil
}

func (c *Client) Devices(ctx context.Context, token string) ([]farm.Device, error) {
	data, _, err := c.request(ctx, http.MethodGet, c.BaseURL+devicesPath, token, nil, nil)
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
