package client

import (
	"context"
	"os"
	"testing"

	"github.com/mikeg/ecobeectl/internal/config"
	"github.com/mikeg/ecobeectl/internal/tokencache"
)

func TestLiveStatus(t *testing.T) {
	email := os.Getenv("ECOBEECTL_LIVE_EMAIL")
	password := os.Getenv("ECOBEECTL_LIVE_PASSWORD")
	thermostatID := os.Getenv("ECOBEECTL_LIVE_THERMOSTAT_ID")
	if email == "" || password == "" || thermostatID == "" {
		t.Skip("set ECOBEECTL_LIVE_EMAIL, ECOBEECTL_LIVE_PASSWORD, and ECOBEECTL_LIVE_THERMOSTAT_ID to run")
	}

	client := New(email, password, thermostatID, DefaultClientID, tokencache.New("ecobeectl-live", config.DefaultCacheDir()))
	if _, err := client.GetThermostat(context.Background(), "runtime", "settings"); err != nil {
		t.Fatal(err)
	}
}
