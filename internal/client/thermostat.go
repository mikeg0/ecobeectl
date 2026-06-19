package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type Thermostat struct {
	Identifier     string         `json:"identifier"`
	Name           string         `json:"name"`
	ModelNumber    string         `json:"modelNumber"`
	ThermostatRev  string         `json:"thermostatRev"`
	ThermostatTime string         `json:"thermostatTime"`
	UTCTime        string         `json:"utcTime"`
	Runtime        Runtime        `json:"runtime"`
	Settings       Settings       `json:"settings"`
	Events         []Event        `json:"events"`
	Program        Program        `json:"program"`
	Weather        Weather        `json:"weather"`
	RemoteSensors  []RemoteSensor `json:"remoteSensors"`
	Alerts         []Alert        `json:"alerts"`
}

type Runtime struct {
	Connected         bool   `json:"connected"`
	ActualTemperature int    `json:"actualTemperature"`
	ActualHumidity    int    `json:"actualHumidity"`
	DesiredHeat       int    `json:"desiredHeat"`
	DesiredCool       int    `json:"desiredCool"`
	DesiredFanMode    string `json:"desiredFanMode"`
	ActualVOC         int    `json:"actualVOC"`
	ActualCO2         int    `json:"actualCO2"`
	ActualAQScore     int    `json:"actualAQScore"`
}

type Settings struct {
	HvacMode         string `json:"hvacMode"`
	FanMinOnTime     int    `json:"fanMinOnTime"`
	HeatCoolMinDelta int    `json:"heatCoolMinDelta"`
	CoolStages       int    `json:"coolStages"`
	HeatStages       int    `json:"heatStages"`
	UseCelsius       bool   `json:"useCelsius"`
	Humidity         string `json:"humidity"`
	HoldAction       string `json:"holdAction"`
}

type Event struct {
	Type           string `json:"type"`
	Name           string `json:"name"`
	Running        bool   `json:"running"`
	HoldClimateRef string `json:"holdClimateRef"`
	CoolHoldTemp   int    `json:"coolHoldTemp"`
	HeatHoldTemp   int    `json:"heatHoldTemp"`
	Fan            string `json:"fan"`
	HoldType       string `json:"holdType"`
	IsIndefinite   bool   `json:"isIndefinite"`
	StartDate      string `json:"startDate"`
	StartTime      string `json:"startTime"`
	EndDate        string `json:"endDate"`
	EndTime        string `json:"endTime"`
}

type Program struct {
	Schedule          [][]string `json:"schedule"`
	Climates          []Climate  `json:"climates"`
	CurrentClimateRef string     `json:"currentClimateRef"`
}

type Climate struct {
	Name       string `json:"name"`
	ClimateRef string `json:"climateRef"`
	CoolTemp   int    `json:"coolTemp"`
	HeatTemp   int    `json:"heatTemp"`
	IsOccupied bool   `json:"isOccupied"`
}

type RemoteSensor struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Type       string       `json:"type"`
	InUse      bool         `json:"inUse"`
	Capability []Capability `json:"capability"`
}

type Capability struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Weather struct {
	Forecasts []Forecast `json:"forecasts"`
}

type Forecast struct {
	WeatherSymbol    int    `json:"weatherSymbol"`
	DateTime         string `json:"dateTime"`
	Condition        string `json:"condition"`
	Temperature      int    `json:"temperature"`
	RelativeHumidity int    `json:"relativeHumidity"`
	WindSpeed        int    `json:"windSpeed"`
	TempHigh         int    `json:"tempHigh"`
	TempLow          int    `json:"tempLow"`
	Pressure         int    `json:"pressure"`
	Pop              int    `json:"pop"`
}

type Alert struct {
	AckRequired bool   `json:"ackRequired"`
	Date        string `json:"date"`
	Text        string `json:"text"`
	Type        string `json:"type"`
}

type thermostatResponse struct {
	apiEnvelope
	ThermostatList []Thermostat `json:"thermostatList"`
}

func (c *Client) GetThermostat(ctx context.Context, includes ...string) (*Thermostat, error) {
	if err := c.EnsureThermostatID(ctx); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		payload := map[string]any{
			"selection": buildSelection(c.ThermostatID, includes),
		}
		query := url.Values{}
		if err := addJSONQuery(query, "json", payload); err != nil {
			return nil, err
		}
		var resp thermostatResponse
		if err := c.do(ctx, "GET", "/1/thermostat", query, nil, &resp); err != nil {
			retried, retryErr := c.retryInvalidSelection(ctx, err)
			if retryErr != nil {
				return nil, retryErr
			}
			if retried {
				continue
			}
			return nil, err
		}
		if err := checkStatus(resp.Status); err != nil {
			return nil, err
		}
		if len(resp.ThermostatList) == 0 {
			return nil, fmt.Errorf("thermostat not found")
		}
		return &resp.ThermostatList[0], nil
	}
	return nil, fmt.Errorf("thermostat request failed after retry")
}

func (c *Client) SetHvacMode(ctx context.Context, mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "heat", "cool", "auto", "off":
	default:
		return fmt.Errorf("invalid HVAC mode %q", mode)
	}
	if err := c.EnsureThermostatID(ctx); err != nil {
		return err
	}
	body := map[string]any{
		"selection": map[string]any{
			"selectionType":  "thermostats",
			"selectionMatch": c.ThermostatID,
		},
		"thermostat": map[string]any{
			"settings": map[string]any{
				"hvacMode": mode,
			},
		},
	}
	return c.postMutation(ctx, body)
}

func (c *Client) SetTemperatureHold(ctx context.Context, heatTemp, coolTemp int, holdType string) error {
	if holdType == "" {
		holdType = "indefinite"
	}
	if err := c.EnsureThermostatID(ctx); err != nil {
		return err
	}
	body := map[string]any{
		"selection": map[string]any{
			"selectionType":  "thermostats",
			"selectionMatch": c.ThermostatID,
		},
		"functions": []map[string]any{{
			"type": "setHold",
			"params": map[string]any{
				"coolHoldTemp": coolTemp,
				"heatHoldTemp": heatTemp,
				"holdType":     holdType,
			},
		}},
	}
	return c.postMutation(ctx, body)
}

func (c *Client) SetFan(ctx context.Context, fanMode, holdType string) error {
	fanMode = strings.ToLower(strings.TrimSpace(fanMode))
	switch fanMode {
	case "on", "auto":
	default:
		return fmt.Errorf("invalid fan mode %q", fanMode)
	}
	if holdType == "" {
		holdType = "nextTransition"
	}
	t, err := c.GetThermostat(ctx, "runtime", "settings")
	if err != nil {
		return err
	}
	body := map[string]any{
		"selection": map[string]any{
			"selectionType":  "thermostats",
			"selectionMatch": c.ThermostatID,
		},
		"functions": []map[string]any{{
			"type": "setHold",
			"params": map[string]any{
				"coolHoldTemp":          t.Runtime.DesiredCool,
				"heatHoldTemp":          t.Runtime.DesiredHeat,
				"holdType":              holdType,
				"fan":                   fanMode,
				"isTemperatureAbsolute": false,
				"isTemperatureRelative": false,
			},
		}},
	}
	return c.postMutation(ctx, body)
}

func (c *Client) SetClimateHold(ctx context.Context, climateRef, holdType string) error {
	if holdType == "" {
		holdType = "indefinite"
	}
	if err := c.EnsureThermostatID(ctx); err != nil {
		return err
	}
	body := map[string]any{
		"selection": map[string]any{
			"selectionType":  "thermostats",
			"selectionMatch": c.ThermostatID,
		},
		"functions": []map[string]any{{
			"type": "setHold",
			"params": map[string]any{
				"holdType":       holdType,
				"holdClimateRef": climateRef,
			},
		}},
	}
	return c.postMutation(ctx, body)
}

func (c *Client) ResumeProgram(ctx context.Context) error {
	if err := c.EnsureThermostatID(ctx); err != nil {
		return err
	}
	body := map[string]any{
		"selection": map[string]any{
			"selectionType":  "thermostats",
			"selectionMatch": c.ThermostatID,
		},
		"functions": []map[string]any{{
			"type": "resumeProgram",
		}},
	}
	return c.postMutation(ctx, body)
}

func (c *Client) ResolveClimateRef(ctx context.Context, input string) (string, error) {
	t, err := c.GetThermostat(ctx, "program")
	if err != nil {
		return "", err
	}
	input = strings.ToLower(strings.TrimSpace(input))
	for _, climate := range t.Program.Climates {
		if strings.ToLower(climate.ClimateRef) == input || strings.ToLower(climate.Name) == input {
			return climate.ClimateRef, nil
		}
	}
	return "", fmt.Errorf("climate %q was not found on this thermostat", input)
}

func (c *Client) postMutation(ctx context.Context, body any) error {
	query := url.Values{"format": []string{"json"}}
	for attempt := 0; attempt < 2; attempt++ {
		var resp apiEnvelope
		if err := c.do(ctx, "POST", "/1/thermostat", query, body, &resp); err != nil {
			retried, retryErr := c.retryInvalidSelection(ctx, err)
			if retryErr != nil {
				return retryErr
			}
			if retried {
				updateSelectionMatch(body, c.ThermostatID)
				continue
			}
			return err
		}
		return checkStatus(resp.Status)
	}
	return fmt.Errorf("thermostat mutation failed after retry")
}

func (c *Client) retryInvalidSelection(ctx context.Context, requestErr error) (bool, error) {
	if !isInvalidSelectionError(requestErr) {
		return false, nil
	}
	ids, err := c.discoverThermostatIDs(ctx)
	if err != nil {
		return false, err
	}
	switch len(ids) {
	case 0:
		return false, nil
	case 1:
		if ids[0] == c.ThermostatID {
			return false, nil
		}
		c.ThermostatID = ids[0]
		return true, nil
	default:
		if containsString(ids, c.ThermostatID) {
			return false, nil
		}
		return false, fmt.Errorf("configured thermostat_id %q is invalid; available thermostat IDs: %s", c.ThermostatID, strings.Join(ids, ", "))
	}
}

func isInvalidSelectionError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "invalid selection") && strings.Contains(msg, "no thermostats in selection")
}

func updateSelectionMatch(body any, thermostatID string) {
	payload, ok := body.(map[string]any)
	if !ok {
		return
	}
	selection, ok := payload["selection"].(map[string]any)
	if !ok {
		return
	}
	selection["selectionMatch"] = thermostatID
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func buildSelection(thermostatID string, includes []string) map[string]any {
	selection := map[string]any{
		"selectionType":  "thermostats",
		"selectionMatch": thermostatID,
	}
	if len(includes) == 0 {
		includes = []string{"runtime", "settings", "events", "program", "weather", "alerts", "sensors"}
	}
	for _, include := range includes {
		switch strings.ToLower(include) {
		case "runtime":
			selection["includeRuntime"] = true
		case "settings":
			selection["includeSettings"] = true
		case "events":
			selection["includeEvents"] = true
		case "program":
			selection["includeProgram"] = true
		case "weather":
			selection["includeWeather"] = true
		case "alerts":
			selection["includeAlerts"] = true
		case "sensors":
			selection["includeSensors"] = true
		}
	}
	return selection
}

func FtoTenths(f float64) int {
	return int((f * 10) + 0.5)
}

func TenthsToF(t int) float64 {
	return float64(t) / 10
}

func TenthsToC(t int) float64 {
	return (TenthsToF(t) - 32) * 5 / 9
}

func CtoTenths(c float64) int {
	return FtoTenths((c * 9 / 5) + 32)
}

func ParseTemp(value string, defaultCelsius bool) (int, error) {
	value = strings.TrimSpace(strings.ToUpper(value))
	switch {
	case strings.HasSuffix(value, "F"):
		f, err := strconv.ParseFloat(strings.TrimSuffix(value, "F"), 64)
		if err != nil {
			return 0, err
		}
		return FtoTenths(f), nil
	case strings.HasSuffix(value, "C"):
		c, err := strconv.ParseFloat(strings.TrimSuffix(value, "C"), 64)
		if err != nil {
			return 0, err
		}
		return CtoTenths(c), nil
	default:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, err
		}
		if defaultCelsius {
			return CtoTenths(f), nil
		}
		return FtoTenths(f), nil
	}
}

func ValidateHeatCoolDelta(heatTemp, coolTemp, minDelta int) error {
	if minDelta <= 0 {
		return nil
	}
	if coolTemp-heatTemp < minDelta {
		return fmt.Errorf("requested setpoints violate heat/cool minimum delta of %.1fF", TenthsToF(minDelta))
	}
	return nil
}

func FlattenRawData(raw json.RawMessage) ([]map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err == nil {
		return list, nil
	}
	var single map[string]any
	if err := json.Unmarshal(raw, &single); err == nil {
		return []map[string]any{single}, nil
	}
	return nil, fmt.Errorf("unsupported JSON shape")
}
