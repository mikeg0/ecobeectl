package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mikeg/ecobeectl/internal/tokencache"
)

func TestAuthenticateUnauthorizedClientMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		mustWriteFixture(t, w, "auth_unauthorized_client.json")
	}))
	defer server.Close()

	client := New("user@example.com", "secret", "", DefaultClientID, tokencache.New("test", t.TempDir()))
	client.AuthURL = server.URL
	err := client.Authenticate(context.Background())
	if err == nil || !strings.Contains(err.Error(), "client ID may have changed") {
		t.Fatalf("expected client ID guidance, got %v", err)
	}
}

func TestGetUserRetriesAfterUnauthorized(t *testing.T) {
	var authCalls int
	var userCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			authCalls++
			mustWriteFixture(t, w, "auth_success.json")
		case "/1/user":
			userCalls++
			if r.Method != http.MethodGet {
				t.Fatalf("method = %s, want GET", r.Method)
			}
			if got := r.URL.Query().Get("json"); got != `{"userName":"user@example.com"}` {
				t.Fatalf("json query = %q, want userName payload", got)
			}
			if userCalls == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				mustWriteFixture(t, w, "http_401.json")
				return
			}
			mustWriteFixture(t, w, "user.json")
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New("user@example.com", "secret", "", DefaultClientID, tokencache.New("test", t.TempDir()))
	client.APIBaseURL = server.URL
	client.AuthURL = server.URL + "/oauth/token"
	raw, err := client.GetUser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if authCalls != 2 {
		t.Fatalf("auth calls = %d, want 2", authCalls)
	}
	if userCalls != 2 {
		t.Fatalf("user calls = %d, want 2", userCalls)
	}
	if !strings.Contains(string(raw), "user@example.com") {
		t.Fatalf("unexpected user payload %s", raw)
	}
}

func TestGetUserRetriesAfterRateLimit(t *testing.T) {
	defer func(orig func(time.Duration)) { sleep = orig }(sleep)
	sleep = func(time.Duration) {}

	var userCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userCalls++
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if got := r.URL.Query().Get("json"); got != `{"userName":"user@example.com"}` {
			t.Fatalf("json query = %q, want userName payload", got)
		}
		if userCalls < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			mustWriteFixture(t, w, "http_429.json")
			return
		}
		mustWriteFixture(t, w, "user.json")
	}))
	defer server.Close()

	client := New("user@example.com", "", "", DefaultClientID, tokencache.New("test", t.TempDir()))
	client.APIBaseURL = server.URL
	client.token = "cached"
	client.tokenExp = time.Now().Add(time.Hour)
	_, err := client.GetUser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if userCalls != 3 {
		t.Fatalf("user calls = %d, want 3", userCalls)
	}
}

func TestGetThermostatStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mustWriteFixture(t, w, "thermostat_status_error.json")
	}))
	defer server.Close()

	client := New("user@example.com", "", "123456789012", DefaultClientID, tokencache.New("test", t.TempDir()))
	client.APIBaseURL = server.URL
	client.token = "cached"
	client.tokenExp = time.Now().Add(time.Hour)
	_, err := client.GetThermostat(context.Background(), "runtime")
	if err == nil || !strings.Contains(err.Error(), "Selection invalid") {
		t.Fatalf("expected status code error, got %v", err)
	}
}

func TestEnsureThermostatIDUsesHomesDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/graphql":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":{"homes":[{"devices":{"thermostats":[{"id":"531668456552"}],"lightSwitches":[]}}],"unassigned":{"thermostats":[],"lightSwitches":[]}}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New("user@example.com", "", "", DefaultClientID, tokencache.New("test", t.TempDir()))
	client.APIBaseURL = server.URL
	client.GraphQLURL = server.URL + "/graphql"
	client.token = "cached"
	client.tokenExp = time.Now().Add(time.Hour)
	if err := client.EnsureThermostatID(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.ThermostatID != "531668456552" {
		t.Fatalf("ThermostatID = %q, want %q", client.ThermostatID, "531668456552")
	}
}

func TestGetThermostatRetriesWithHomesDiscoveredID(t *testing.T) {
	var thermostatCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/1/thermostat":
			thermostatCalls++
			if strings.Contains(r.URL.RawQuery, "123456789012") {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"status":{"code":9,"message":"Invalid selection. No thermostats in selection. Ensure permissions and selection."}}`))
				return
			}
			mustWriteFixture(t, w, "thermostat.json")
		case "/graphql":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":{"homes":[{"devices":{"thermostats":[{"id":"531668456552"}],"lightSwitches":[]}}],"unassigned":{"thermostats":[],"lightSwitches":[]}}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New("user@example.com", "", "123456789012", DefaultClientID, tokencache.New("test", t.TempDir()))
	client.APIBaseURL = server.URL
	client.GraphQLURL = server.URL + "/graphql"
	client.token = "cached"
	client.tokenExp = time.Now().Add(time.Hour)
	thermostat, err := client.GetThermostat(context.Background(), "runtime")
	if err != nil {
		t.Fatal(err)
	}
	if thermostat.Identifier != "123456789012" {
		t.Fatalf("thermostat identifier = %q, want fixture value %q", thermostat.Identifier, "123456789012")
	}
	if client.ThermostatID != "531668456552" {
		t.Fatalf("ThermostatID = %q, want recovered %q", client.ThermostatID, "531668456552")
	}
	if thermostatCalls != 2 {
		t.Fatalf("thermostat calls = %d, want 2", thermostatCalls)
	}
}

func TestParseTempAndValidateDelta(t *testing.T) {
	if got, want := mustParseTemp(t, "73", false), 730; got != want {
		t.Fatalf("ParseTemp(\"73\") = %d, want %d", got, want)
	}
	if got, want := mustParseTemp(t, "22.0C", false), 716; got != want {
		t.Fatalf("ParseTemp(\"22.0C\") = %d, want %d", got, want)
	}
	if err := ValidateHeatCoolDelta(700, 710, 20); err == nil {
		t.Fatalf("expected delta validation error")
	}
}

func TestSetFanPreservesTempsAndHoldType(t *testing.T) {
	var posted map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			mustWriteFixture(t, w, "thermostat.json")
		case http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
				t.Fatal(err)
			}
			w.Write([]byte(`{"status":{"code":0,"message":""}}`))
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	client := New("user@example.com", "", "123456789012", DefaultClientID, tokencache.New("test", t.TempDir()))
	client.APIBaseURL = server.URL
	client.token = "cached"
	client.tokenExp = time.Now().Add(time.Hour)
	if err := client.SetFan(context.Background(), "on", "nextTransition"); err != nil {
		t.Fatal(err)
	}
	functions := posted["functions"].([]any)
	params := functions[0].(map[string]any)["params"].(map[string]any)
	if got := params["holdType"]; got != "nextTransition" {
		t.Fatalf("holdType = %v, want nextTransition", got)
	}
	if got := int(params["heatHoldTemp"].(float64)); got != 680 {
		t.Fatalf("heatHoldTemp = %d, want 680", got)
	}
	if got := int(params["coolHoldTemp"].(float64)); got != 745 {
		t.Fatalf("coolHoldTemp = %d, want 745", got)
	}
}

func TestClientIDDetectorHomepageAndFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/en-us/":
			w.Write([]byte(`<a href="https://auth.ecobee.com/authorize?client_id=homepageclient123">Sign in</a>`))
		case "/no-homepage":
			w.Write([]byte(`<html></html>`))
		case "/consumerportal/index.html":
			w.Write([]byte(`<script src="scripts.abc123.js"></script>`))
		case "/consumerportal/scripts.abc123.js":
			w.Write([]byte(`var client_id="bundleclient456";`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	detector := ClientIDDetector{
		HomepageURL:       server.URL + "/en-us/",
		ConsumerPortalURL: server.URL + "/consumerportal/index.html",
	}
	id, source, err := detector.Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if id != "homepageclient123" || source != "homepage" {
		t.Fatalf("unexpected homepage detection %q from %q", id, source)
	}

	detector.HomepageURL = server.URL + "/no-homepage"
	id, source, err = detector.Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if id != "bundleclient456" || source != "consumer-portal-js" {
		t.Fatalf("unexpected fallback detection %q from %q", id, source)
	}
}

func mustParseTemp(t *testing.T, value string, useCelsius bool) int {
	t.Helper()
	got, err := ParseTemp(value, useCelsius)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func mustWriteFixture(t *testing.T, w http.ResponseWriter, name string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
