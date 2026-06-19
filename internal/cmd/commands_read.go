package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	ecoclient "github.com/mikeg/ecobeectl/internal/client"
)

func newWeatherCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "weather",
		Short: "Show weather forecast",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(cmd, true)
			if err != nil {
				return err
			}
			ctx, cancel := contextWithTimeout()
			defer cancel()
			t, err := client.GetThermostat(ctx, "weather")
			if err != nil {
				return err
			}
			rows := make([]map[string]any, 0, len(t.Weather.Forecasts))
			for _, forecast := range t.Weather.Forecasts {
				rows = append(rows, map[string]any{
					"datetime":      forecast.DateTime,
					"condition":     forecast.Condition,
					"temperature":   displayForecastTemp(forecast.Temperature),
					"temp_high":     displayForecastTemp(forecast.TempHigh),
					"temp_low":      displayForecastTemp(forecast.TempLow),
					"humidity":      fmt.Sprintf("%d%%", forecast.RelativeHumidity),
					"wind_speed":    forecast.WindSpeed,
					"precip_chance": forecast.Pop,
				})
			}
			return render(cmd, []string{"datetime", "condition", "temperature", "temp_high", "temp_low", "humidity", "wind_speed", "precip_chance"}, rows)
		},
	}
}

func newSensorsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sensors",
		Short: "Show remote sensor readings",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(cmd, true)
			if err != nil {
				return err
			}
			ctx, cancel := contextWithTimeout()
			defer cancel()
			t, err := client.GetThermostat(ctx, "sensors")
			if err != nil {
				return err
			}
			rows := make([]map[string]any, 0, len(t.RemoteSensors))
			for _, sensor := range t.RemoteSensors {
				row := map[string]any{
					"id":     sensor.ID,
					"name":   sensor.Name,
					"type":   sensor.Type,
					"in_use": sensor.InUse,
				}
				for _, capability := range sensor.Capability {
					switch strings.ToLower(capability.Type) {
					case "temperature":
						row["temperature"] = parseSensorTemperature(capability.Value)
					case "occupancy":
						row["occupancy"] = capability.Value
					default:
						row[capability.Type] = capability.Value
					}
				}
				rows = append(rows, row)
			}
			return render(cmd, []string{"id", "name", "type", "in_use", "temperature", "occupancy"}, rows)
		},
	}
}

func newScheduleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "schedule",
		Short: "Show program schedule",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(cmd, true)
			if err != nil {
				return err
			}
			ctx, cancel := contextWithTimeout()
			defer cancel()
			t, err := client.GetThermostat(ctx, "program")
			if err != nil {
				return err
			}
			rows := make([]map[string]any, 0)
			for dayIndex, slots := range t.Program.Schedule {
				lastClimate := ""
				for slotIndex, climateRef := range slots {
					if climateRef == lastClimate {
						continue
					}
					rows = append(rows, map[string]any{
						"day":         dayLabel(dayIndex),
						"time":        slotTime(slotIndex),
						"climate_ref": climateRef,
						"current":     climateRef == t.Program.CurrentClimateRef,
					})
					lastClimate = climateRef
				}
			}
			if len(rows) == 0 {
				for _, climate := range t.Program.Climates {
					rows = append(rows, map[string]any{
						"climate_ref": climate.ClimateRef,
						"name":        climate.Name,
						"heat":        displayTemp(climate.HeatTemp),
						"cool":        displayTemp(climate.CoolTemp),
						"current":     climate.ClimateRef == t.Program.CurrentClimateRef,
					})
				}
				return render(cmd, []string{"climate_ref", "name", "heat", "cool", "current"}, rows)
			}
			return render(cmd, []string{"day", "time", "climate_ref", "current"}, rows)
		},
	}
}

func newEventsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "events",
		Short: "Show thermostat events (holds, vacations, demand response)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(cmd, true)
			if err != nil {
				return err
			}
			ctx, cancel := contextWithTimeout()
			defer cancel()
			t, err := client.GetThermostat(ctx, "events")
			if err != nil {
				return err
			}
			return render(cmd, eventHeaders, eventRows(t))
		},
	}
}

var eventHeaders = []string{"type", "name", "running", "hold_climate_ref", "hold_type", "is_indefinite", "heat", "cool", "fan", "start", "end"}

func eventRows(t *ecoclient.Thermostat) []map[string]any {
	rows := make([]map[string]any, 0, len(t.Events))
	for i := range t.Events {
		event := t.Events[i]
		rows = append(rows, map[string]any{
			"type":             event.Type,
			"name":             event.Name,
			"running":          event.Running,
			"hold_climate_ref": event.HoldClimateRef,
			"hold_type":        holdTypeLabel(&event),
			"is_indefinite":    event.IsIndefinite,
			"heat":             displayTemp(event.HeatHoldTemp),
			"cool":             displayTemp(event.CoolHoldTemp),
			"fan":              event.Fan,
			"start":            strings.TrimSpace(event.StartDate + " " + event.StartTime),
			"end":              strings.TrimSpace(event.EndDate + " " + event.EndTime),
		})
	}
	return rows
}

func newAlertsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "alerts",
		Short: "Show thermostat alerts",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(cmd, true)
			if err != nil {
				return err
			}
			ctx, cancel := contextWithTimeout()
			defer cancel()
			t, err := client.GetThermostat(ctx, "alerts")
			if err != nil {
				return err
			}
			rows := make([]map[string]any, 0, len(t.Alerts))
			for _, alert := range t.Alerts {
				rows = append(rows, map[string]any{
					"type":         alert.Type,
					"text":         alert.Text,
					"date":         alert.Date,
					"ack_required": alert.AckRequired,
				})
			}
			return render(cmd, []string{"type", "text", "date", "ack_required"}, rows)
		},
	}
}

func newDevicesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "devices",
		Short: "List registered devices",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(cmd, true)
			if err != nil {
				return err
			}
			ctx, cancel := contextWithTimeout()
			defer cancel()
			devices, err := client.ListDevices(ctx)
			if err != nil {
				return err
			}
			rows := make([]map[string]any, 0, len(devices))
			for _, device := range devices {
				rows = append(rows, map[string]any{
					"identifier": device.Identifier(),
					"name":       device.Name(),
					"type":       device.Type(),
					"raw":        device.Raw,
				})
			}
			return render(cmd, []string{"identifier", "name", "type", "raw"}, rows)
		},
	}
}

func newHomesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "homes",
		Short: "Show home structure",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(cmd, true)
			if err != nil {
				return err
			}
			ctx, cancel := contextWithTimeout()
			defer cancel()
			raw, err := client.GetHomes(ctx)
			if err != nil {
				return err
			}
			headers, rows, err := flattenRawRows(raw)
			if err != nil {
				return err
			}
			return render(cmd, headers, rows)
		},
	}
}

func newEnergyCmd() *cobra.Command {
	var startDate string
	var endDate string
	cmd := &cobra.Command{
		Use:   "energy",
		Short: "Show Energy IQ data",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(cmd, true)
			if err != nil {
				return err
			}
			if startDate == "" || endDate == "" {
				now := time.Now().In(nowInConfiguredLocation())
				if endDate == "" {
					endDate = now.Format("2006-01-02")
				}
				if startDate == "" {
					startDate = now.AddDate(0, 0, -7).Format("2006-01-02")
				}
			}
			ctx, cancel := contextWithTimeout()
			defer cancel()
			raw, err := client.GetEnergyReport(ctx, startDate, endDate)
			if err != nil {
				return err
			}
			headers, rows, err := flattenRawRows(raw)
			if err != nil || len(rows) == 0 {
				row := map[string]any{"start_date": startDate, "end_date": endDate}
				if len(raw) > 0 {
					row["data"] = json.RawMessage(raw)
				}
				return render(cmd, []string{"start_date", "end_date", "data"}, []map[string]any{row})
			}
			return render(cmd, headers, rows)
		},
	}
	cmd.Flags().StringVar(&startDate, "start", "", "report start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&endDate, "end", "", "report end date (YYYY-MM-DD)")
	return cmd
}

func newAirQualityCmd() *cobra.Command {
	var startDate string
	var endDate string
	cmd := &cobra.Command{
		Use:   "air-quality",
		Short: "Show air quality runtime report",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(cmd, true)
			if err != nil {
				return err
			}
			if startDate == "" || endDate == "" {
				now := time.Now().In(nowInConfiguredLocation())
				if endDate == "" {
					endDate = now.Format("2006-01-02")
				}
				if startDate == "" {
					startDate = now.AddDate(0, 0, -7).Format("2006-01-02")
				}
			}
			ctx, cancel := contextWithTimeout()
			defer cancel()
			samples, err := client.GetAirQualityReport(ctx, startDate, endDate)
			if err != nil {
				return err
			}
			rows := make([]map[string]any, 0, len(samples))
			for _, sample := range samples {
				name := sample.SensorName
				if name == "" {
					name = sample.SensorID
				}
				rows = append(rows, map[string]any{
					"date":         sample.Date,
					"time":         sample.Time,
					"sensor":       name,
					"air_quality":  sample.Values["airQuality"],
					"accuracy":     sample.Values["airQualityAccuracy"],
					"co2_ppm":      sample.Values["co2PPM"],
					"voc_ppm":      sample.Values["vocPPM"],
					"air_pressure": sample.Values["airPressure"],
				})
			}
			return render(cmd, []string{"date", "time", "sensor", "air_quality", "accuracy", "co2_ppm", "voc_ppm", "air_pressure"}, rows)
		},
	}
	cmd.Flags().StringVar(&startDate, "start", "", "report start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&endDate, "end", "", "report end date (YYYY-MM-DD)")
	return cmd
}

func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show user account information",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(cmd, true)
			if err != nil {
				return err
			}
			ctx, cancel := contextWithTimeout()
			defer cancel()
			raw, err := client.GetUser(ctx)
			if err != nil {
				return err
			}
			headers, rows, err := flattenRawRows(raw)
			if err != nil {
				return err
			}
			return render(cmd, headers, rows)
		},
	}
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear cached tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(cmd, false)
			if err != nil {
				return err
			}
			if strings.TrimSpace(client.Email) == "" {
				return fmt.Errorf("logout requires email so the cached identity can be selected")
			}
			if err := client.ClearCachedToken(); err != nil {
				return err
			}
			return render(cmd, []string{"action", "email"}, []map[string]any{{"action": "cleared", "email": client.Email}})
		},
	}
}
