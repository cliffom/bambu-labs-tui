package cloud

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fake(t *testing.T, handler func(*http.Request) (int, string, http.Header)) *Client {
	t.Helper()
	c := New("global")
	c.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		code, body, h := handler(r)
		return &http.Response{StatusCode: code, Header: h, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	return c
}

func TestLoginAndDiscovery(t *testing.T) {
	ctx := context.Background()
	c := fake(t, func(r *http.Request) (int, string, http.Header) {
		switch r.URL.Path {
		case "/v1/user-service/user/login":
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), `"code"`) {
				if strings.Contains(string(body), `"password"`) {
					t.Error("password and code submitted together")
				}
				return 200, `{"accessToken":"opaque-test-token"}`, nil
			}
			return 200, `{"loginType":"verifyCode"}`, nil
		case "/v1/design-user-service/my/preference":
			if r.Header.Get("Authorization") != "Bearer opaque-test-token" {
				t.Error("authorization")
			}
			return 200, `{"uid":12345}`, nil
		case "/v1/iot-service/api/user/bind":
			return 200, `{"devices":[{"dev_id":"fixture","name":"H2D","dev_product_name":"H2D","online":true}]}`, nil
		default:
			t.Fatalf("unexpected endpoint %s", r.URL.Path)
			return 500, "", nil
		}
	})
	r, err := c.Login(ctx, "test@example.invalid", "test-password", "")
	if err != nil || r.LoginType != "verifyCode" {
		t.Fatal(r, err)
	}
	r, err = c.Login(ctx, "test@example.invalid", "", "123456")
	if err != nil {
		t.Fatal(err)
	}
	session, err := c.Session(ctx, r.AccessToken)
	if err != nil || session.Username != "u_12345" {
		t.Fatal(session, err)
	}
	devices, err := c.Devices(ctx, session.Token)
	if err != nil || len(devices) != 1 || devices[0].Model != "H2D" {
		t.Fatal(devices, err)
	}
}

func TestAuthenticatorCookieFlow(t *testing.T) {
	ctx := context.Background()
	c := fake(t, func(r *http.Request) (int, string, http.Header) {
		switch r.URL.Path {
		case "/api/csrf":
			return 204, "", http.Header{"Set-Cookie": {`bbl_csrf_token=csrf-fixture; Path=/; Secure`}}
		case "/api/sign-in/tfa":
			cookie, err := r.Cookie("bbl_csrf_token")
			if err != nil || cookie.Value != "csrf-fixture" || r.Header.Get("X-Bbl-Csrf-Token") != "csrf-fixture" {
				t.Error("CSRF cookie/header missing")
			}
			return 200, `{}`, http.Header{"Set-Cookie": {`token=token-fixture; Path=/; Secure`}}
		default:
			t.Fatal(r.URL.Path)
			return 500, "", nil
		}
	})
	token, err := c.TwoFactor(ctx, "challenge-fixture", "123456")
	if err != nil || token != "token-fixture" {
		t.Fatal(token, err)
	}
}

func TestErrorsNeverExposeResponseSecrets(t *testing.T) {
	for _, tc := range []struct {
		code int
		want error
	}{{401, ErrAuth}, {403, ErrBlocked}, {429, ErrRateLimited}, {500, nil}, {200, nil}} {
		t.Run(http.StatusText(tc.code), func(t *testing.T) {
			c := fake(t, func(*http.Request) (int, string, http.Header) {
				return tc.code, `{"code":9,"message":"sensitive-account-value","accessToken":"sensitive-token"}`, nil
			})
			_, err := c.Devices(context.Background(), "secret")
			if err == nil {
				t.Fatal("expected error")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
			if strings.Contains(err.Error(), "sensitive") || strings.Contains(err.Error(), "secret") {
				t.Fatal("secret in error", err)
			}
		})
	}
}

func TestRegionsAndInvalidIdentity(t *testing.T) {
	c := New("china")
	if !strings.Contains(c.BaseURL, ".cn") || c.MQTTAddress != "cn.mqtt.bambulab.com:8883" {
		t.Fatal("region")
	}
	c = fake(t, func(*http.Request) (int, string, http.Header) { return 200, `{"uid":"bad/identity"}`, nil })
	if _, err := c.Session(context.Background(), "test"); err == nil {
		t.Fatal("accepted invalid identity")
	}
}
