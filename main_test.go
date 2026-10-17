package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestHandleGenerateURLAllowsCrossOriginRequests(t *testing.T) {
	app := &OAuthApp{
		config: &Config{
			ClientID: "test-client-id",
			AppURL:   "https://example.com",
		},
		logger: logrus.New(),
	}
	request := httptest.NewRequest(http.MethodGet, "/generate", nil)
	response := httptest.NewRecorder()

	app.handleGenerateURL(response, request)

	if got, want := response.Header().Get("Access-Control-Allow-Origin"), "*"; got != want {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, want)
	}
	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "https://github.com/login/oauth/authorize?") {
		t.Fatalf("response does not contain generated GitHub authorization URL: %q", response.Body.String())
	}
}

func TestRemoteClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		want       string
	}{
		{
			name:       "forwarded for",
			remoteAddr: "127.0.0.1:3000",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.10, 10.0.0.1"},
			want:       "203.0.113.10",
		},
		{
			name:       "real ip fallback",
			remoteAddr: "127.0.0.1:3000",
			headers:    map[string]string{"X-Real-IP": "203.0.113.11"},
			want:       "203.0.113.11",
		},
		{
			name:       "invalid forwarded headers fallback to remote addr",
			remoteAddr: "[::1]:3000",
			headers:    map[string]string{"X-Forwarded-For": "unknown", "X-Real-IP": "invalid"},
			want:       "::1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "/", nil)
			request.RemoteAddr = test.remoteAddr
			for name, value := range test.headers {
				request.Header.Set(name, value)
			}

			if got := remoteClientIP(request); got != test.want {
				t.Errorf("remoteClientIP() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestFormatTelegramNotificationIncludesCountryBesideIP(t *testing.T) {
	message := formatTelegramNotification(
		"octocat",
		123,
		"bearer",
		time.Time{},
		nil,
		"203.0.113.10",
		"US",
		"abcd",
		time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	)

	if !strings.Contains(message, "Client IP: 203.0.113.10 (US)") {
		t.Fatalf("notification does not include country code beside IP: %q", message)
	}
}

func TestFormatClientIPWithoutCountry(t *testing.T) {
	if got, want := formatClientIP("203.0.113.10", ""), "203.0.113.10 (country unavailable)"; got != want {
		t.Fatalf("formatClientIP() = %q, want %q", got, want)
	}
}

func TestCountryCodeFromIPWho(t *testing.T) {
	countryCode, err := countryCodeFromIPWho(strings.NewReader(`{"success":true,"country_code":"us"}`))
	if err != nil {
		t.Fatalf("countryCodeFromIPWho() error = %v", err)
	}
	if countryCode != "US" {
		t.Fatalf("countryCodeFromIPWho() = %q, want %q", countryCode, "US")
	}
}

func TestCountryCodeFromIPWhoRejectsFailedLookup(t *testing.T) {
	_, err := countryCodeFromIPWho(strings.NewReader(`{"success":false,"message":"Reserved range"}`))
	if err == nil {
		t.Fatal("countryCodeFromIPWho() error = nil, want an error")
	}
}
