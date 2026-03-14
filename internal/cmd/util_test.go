package cmd

import (
	"testing"

	"github.com/mikeg/ecobeectl/internal/config"
)

func TestDisplayForecastTemp(t *testing.T) {
	original := state
	t.Cleanup(func() {
		state = original
	})

	state.loaded = config.Loaded{Config: config.Config{UseCelsius: false}}
	if got, want := displayForecastTemp(740), "74.0F"; got != want {
		t.Fatalf("displayForecastTemp(740) = %q, want %q", got, want)
	}
	if got := displayForecastTemp(-5002); got != "" {
		t.Fatalf("displayForecastTemp(-5002) = %q, want empty string", got)
	}

	state.loaded = config.Loaded{Config: config.Config{UseCelsius: true}}
	if got, want := displayForecastTemp(740), "23.3C"; got != want {
		t.Fatalf("displayForecastTemp(740) in Celsius = %q, want %q", got, want)
	}
	if got := displayForecastTemp(-5002); got != "" {
		t.Fatalf("displayForecastTemp(-5002) in Celsius = %q, want empty string", got)
	}
}
