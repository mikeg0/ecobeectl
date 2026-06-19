package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	ecoclient "github.com/mikeg/ecobeectl/internal/client"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show thermostat status",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(cmd, true)
			if err != nil {
				return err
			}
			ctx, cancel := contextWithTimeout()
			defer cancel()
			t, err := client.GetThermostat(ctx, "runtime", "settings", "events", "program", "weather")
			if err != nil {
				return err
			}
			outsideTemp := ""
			if len(t.Weather.Forecasts) > 0 {
				outsideTemp = displayForecastTemp(t.Weather.Forecasts[0].Temperature)
			}
			row := map[string]any{
				"name":             t.Name,
				"identifier":       t.Identifier,
				"connected":        t.Runtime.Connected,
				"current_temp":     displayTemp(t.Runtime.ActualTemperature),
				"outside_temp":     outsideTemp,
				"current_humidity": fmt.Sprintf("%d%%", t.Runtime.ActualHumidity),
				"hvac_mode":        t.Settings.HvacMode,
				"desired_heat":     displayTemp(t.Runtime.DesiredHeat),
				"desired_cool":     displayTemp(t.Runtime.DesiredCool),
				"fan_mode":         t.Runtime.DesiredFanMode,
				"hold":             activeHoldStatus(t),
				"air_quality":      t.Runtime.ActualAQScore,
			}
			return render(cmd, []string{"name", "identifier", "connected", "current_temp", "outside_temp", "current_humidity", "hvac_mode", "desired_heat", "desired_cool", "fan_mode", "hold", "air_quality"}, []map[string]any{row})
		},
	}
}

func newModeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mode <heat|cool|auto|off>",
		Short: "Set HVAC mode",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(cmd, true)
			if err != nil {
				return err
			}
			ctx, cancel := contextWithTimeout()
			defer cancel()
			mode := strings.ToLower(args[0])
			if err := client.SetHvacMode(ctx, mode); err != nil {
				return err
			}
			return render(cmd, []string{"action", "hvac_mode"}, []map[string]any{{"action": "updated", "hvac_mode": mode}})
		},
	}
}

func newTempCmd() *cobra.Command {
	var heatValue string
	var coolValue string
	var holdType string
	cmd := &cobra.Command{
		Use:   "temp [value]",
		Short: "Set temperature hold",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(cmd, true)
			if err != nil {
				return err
			}
			ctx, cancel := contextWithTimeout()
			defer cancel()
			t, err := client.GetThermostat(ctx, "runtime", "settings")
			if err != nil {
				return err
			}

			var heatTemp, coolTemp int
			switch strings.ToLower(t.Settings.HvacMode) {
			case "off":
				return fmt.Errorf("temp cannot be used while the thermostat is in off mode")
			case "auto":
				if heatValue == "" || coolValue == "" {
					return fmt.Errorf("temp requires both --heat and --cool while the thermostat is in auto mode")
				}
				heatTemp, err = ecoclient.ParseTemp(heatValue, state.loaded.Config.UseCelsius)
				if err != nil {
					return fmt.Errorf("parse --heat: %w", err)
				}
				coolTemp, err = ecoclient.ParseTemp(coolValue, state.loaded.Config.UseCelsius)
				if err != nil {
					return fmt.Errorf("parse --cool: %w", err)
				}
			case "heat":
				value := firstNonEmpty(firstArg(args), heatValue)
				if value == "" {
					return fmt.Errorf("temp requires a target value in heat mode")
				}
				heatTemp, err = ecoclient.ParseTemp(value, state.loaded.Config.UseCelsius)
				if err != nil {
					return err
				}
				coolTemp = t.Runtime.DesiredCool
			case "cool":
				value := firstNonEmpty(firstArg(args), coolValue)
				if value == "" {
					return fmt.Errorf("temp requires a target value in cool mode")
				}
				coolTemp, err = ecoclient.ParseTemp(value, state.loaded.Config.UseCelsius)
				if err != nil {
					return err
				}
				heatTemp = t.Runtime.DesiredHeat
			default:
				return fmt.Errorf("temp does not support current hvac mode %q", t.Settings.HvacMode)
			}

			if err := ecoclient.ValidateHeatCoolDelta(heatTemp, coolTemp, t.Settings.HeatCoolMinDelta); err != nil {
				return err
			}
			if err := client.SetTemperatureHold(ctx, heatTemp, coolTemp, holdType); err != nil {
				return err
			}
			return render(cmd, []string{"action", "heat", "cool", "hold_type"}, []map[string]any{{
				"action":    "updated",
				"heat":      displayTemp(heatTemp),
				"cool":      displayTemp(coolTemp),
				"hold_type": firstNonEmpty(holdType, "indefinite"),
			}})
		},
	}
	cmd.Flags().StringVar(&heatValue, "heat", "", "heat setpoint for auto mode")
	cmd.Flags().StringVar(&coolValue, "cool", "", "cool setpoint for auto mode")
	cmd.Flags().StringVar(&holdType, "hold-type", "indefinite", "hold type")
	return cmd
}

func newFanCmd() *cobra.Command {
	var holdType string
	cmd := &cobra.Command{
		Use:   "fan <on|auto>",
		Short: "Set fan mode",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(cmd, true)
			if err != nil {
				return err
			}
			ctx, cancel := contextWithTimeout()
			defer cancel()
			fanMode := strings.ToLower(args[0])
			if err := client.SetFan(ctx, fanMode, holdType); err != nil {
				return err
			}
			return render(cmd, []string{"action", "fan_mode", "hold_type"}, []map[string]any{{"action": "updated", "fan_mode": fanMode, "hold_type": holdType}})
		},
	}
	cmd.Flags().StringVar(&holdType, "hold-type", "nextTransition", "hold type")
	return cmd
}

func newHoldCmd() *cobra.Command {
	var holdType string
	cmd := &cobra.Command{
		Use:   "hold <climate>",
		Short: "Set climate hold",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(cmd, true)
			if err != nil {
				return err
			}
			ctx, cancel := contextWithTimeout()
			defer cancel()
			climateRef, err := client.ResolveClimateRef(ctx, args[0])
			if err != nil {
				return err
			}
			if err := client.SetClimateHold(ctx, climateRef, holdType); err != nil {
				return err
			}
			return render(cmd, []string{"action", "climate", "hold_type"}, []map[string]any{{"action": "updated", "climate": climateRef, "hold_type": holdType}})
		},
	}
	cmd.Flags().StringVar(&holdType, "hold-type", "indefinite", "hold type")
	return cmd
}

func newResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume",
		Short: "Resume scheduled program",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(cmd, true)
			if err != nil {
				return err
			}
			ctx, cancel := contextWithTimeout()
			defer cancel()
			if err := client.ResumeProgram(ctx); err != nil {
				return err
			}
			return render(cmd, []string{"action"}, []map[string]any{{"action": "resumed"}})
		},
	}
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
