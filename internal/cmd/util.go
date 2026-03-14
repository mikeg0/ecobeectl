package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/term"

	ecoclient "github.com/mikeg/ecobeectl/internal/client"
)

func promptPassword(prompt string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("password is required and stdin is not interactive; set password in config, use ECOBEECTL_PASSWORD, or pass --password")
	}
	fmt.Fprint(os.Stderr, prompt)
	bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bytes)), nil
}

func displayTemp(t int) string {
	if state.loaded.Config.UseCelsius {
		return fmt.Sprintf("%.1fC", ecoclient.TenthsToC(t))
	}
	return fmt.Sprintf("%.1fF", ecoclient.TenthsToF(t))
}

func displayForecastTemp(t int) string {
	if t <= -5000 {
		return ""
	}
	return displayTemp(t)
}

func activeHoldStatus(t *ecoclient.Thermostat) string {
	for _, event := range t.Events {
		if event.Running {
			if event.HoldClimateRef != "" {
				return event.HoldClimateRef
			}
			if event.Name != "" {
				return event.Name
			}
			if event.Type != "" {
				return event.Type
			}
		}
	}
	if t.Program.CurrentClimateRef != "" {
		return t.Program.CurrentClimateRef
	}
	return ""
}

func flattenRawRows(raw json.RawMessage) ([]string, []map[string]any, error) {
	rows, err := ecoclient.FlattenRawData(raw)
	if err != nil {
		return nil, nil, err
	}
	headers := collectHeaders(rows)
	return headers, rows, nil
}

func collectHeaders(rows []map[string]any) []string {
	keys := map[string]struct{}{}
	for _, row := range rows {
		for key := range row {
			keys[key] = struct{}{}
		}
	}
	headers := make([]string, 0, len(keys))
	for key := range keys {
		headers = append(headers, key)
	}
	sort.Strings(headers)
	return headers
}

func dayLabel(index int) string {
	days := []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
	if index >= 0 && index < len(days) {
		return days[index]
	}
	return fmt.Sprintf("day-%d", index+1)
}

func slotTime(index int) string {
	return fmt.Sprintf("%02d:%02d", index/2, (index%2)*30)
}

func parseSensorTemperature(value string) string {
	if value == "" {
		return ""
	}
	tenths, err := strconv.Atoi(value)
	if err != nil {
		return value
	}
	return displayTemp(tenths)
}
