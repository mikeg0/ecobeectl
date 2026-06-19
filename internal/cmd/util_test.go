package cmd

import (
	"testing"

	ecoclient "github.com/mikeg/ecobeectl/internal/client"
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

func TestActiveHoldEvent(t *testing.T) {
	therm := &ecoclient.Thermostat{
		Events: []ecoclient.Event{
			{Type: "hold", Running: false, HoldClimateRef: "away"},
			{Type: "hold", Running: true, HoldClimateRef: "home", HoldType: "nextTransition"},
		},
	}
	event := activeHoldEvent(therm)
	if event == nil {
		t.Fatal("activeHoldEvent returned nil, want the running event")
	}
	if event.HoldClimateRef != "home" {
		t.Fatalf("activeHoldEvent returned hold for %q, want %q", event.HoldClimateRef, "home")
	}

	if activeHoldEvent(&ecoclient.Thermostat{}) != nil {
		t.Fatal("activeHoldEvent returned non-nil for a thermostat with no events")
	}
}

func TestHoldEnds(t *testing.T) {
	if got, want := holdEnds(&ecoclient.Event{IsIndefinite: true}), "indefinite"; got != want {
		t.Fatalf("holdEnds(indefinite) = %q, want %q", got, want)
	}
	if got, want := holdEnds(&ecoclient.Event{EndDate: "2026-06-19", EndTime: "17:30:00"}), "2026-06-19 17:30:00"; got != want {
		t.Fatalf("holdEnds = %q, want %q", got, want)
	}
	if got, want := holdEnds(&ecoclient.Event{EndDate: "2026-06-19"}), "2026-06-19"; got != want {
		t.Fatalf("holdEnds without time = %q, want %q", got, want)
	}
}

func TestHoldTypeLabel(t *testing.T) {
	// An explicit holdType is reported verbatim.
	if got, want := holdTypeLabel(&ecoclient.Event{HoldType: "nextTransition"}), "nextTransition"; got != want {
		t.Fatalf("holdTypeLabel(explicit) = %q, want %q", got, want)
	}
	// A truly indefinite event with no end reports "indefinite".
	if got, want := holdTypeLabel(&ecoclient.Event{IsIndefinite: true}), "indefinite"; got != want {
		t.Fatalf("holdTypeLabel(indefinite) = %q, want %q", got, want)
	}
	// A system event (empty holdType) with a concrete end must NOT be labelled
	// "indefinite" — that contradicted the end time shown alongside it.
	event := &ecoclient.Event{EndDate: "2026-06-19", EndTime: "15:00:00"}
	if got, want := holdTypeLabel(event), "dateTime"; got != want {
		t.Fatalf("holdTypeLabel(timed system event) = %q, want %q", got, want)
	}
}

func TestEventRows(t *testing.T) {
	therm := &ecoclient.Thermostat{
		Events: []ecoclient.Event{
			{
				Type:         "touPrecool",
				Name:         "prc150000",
				Running:      true,
				HeatHoldTemp: 450,
				CoolHoldTemp: 650,
				Fan:          "on",
				StartDate:    "2026-06-19",
				StartTime:    "13:20:00",
				EndDate:      "2026-06-19",
				EndTime:      "15:00:00",
			},
		},
	}

	original := state
	t.Cleanup(func() { state = original })
	state.loaded = config.Loaded{Config: config.Config{UseCelsius: false}}

	rows := eventRows(therm)
	if len(rows) != 1 {
		t.Fatalf("eventRows returned %d rows, want 1", len(rows))
	}
	row := rows[0]
	checks := map[string]any{
		"type":          "touPrecool",
		"name":          "prc150000",
		"running":       true,
		"hold_type":     "dateTime",
		"is_indefinite": false,
		"heat":          "45.0F",
		"cool":          "65.0F",
		"start":         "2026-06-19 13:20:00",
		"end":           "2026-06-19 15:00:00",
	}
	for key, want := range checks {
		if got := row[key]; got != want {
			t.Errorf("eventRows row[%q] = %v, want %v", key, got, want)
		}
	}
}
