package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	ecoclient "github.com/mikeg/ecobeectl/internal/client"
	"github.com/mikeg/ecobeectl/internal/config"
	"github.com/mikeg/ecobeectl/internal/output"
	"github.com/mikeg/ecobeectl/internal/tokencache"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

type runtimeState struct {
	loaded config.Loaded
}

var state runtimeState

func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "ecobeectl",
		Short:         "Control ecobee thermostats from the command line",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := config.Load(getStringFlag(cmd, "config"), cmd.Flags())
			if err != nil {
				return err
			}
			state.loaded = loaded
			if loaded.Config.Verbose {
				log.SetLevel(log.DebugLevel)
			} else {
				log.SetLevel(log.InfoLevel)
			}
			for _, warning := range loaded.Warnings {
				log.Warn(warning)
			}
			return nil
		},
	}

	rootCmd.PersistentFlags().String("config", config.DefaultConfigPath(), "config file path")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "enable debug logging")
	rootCmd.PersistentFlags().String("email", "", "ecobee account email")
	rootCmd.PersistentFlags().String("password", "", "ecobee account password")
	rootCmd.PersistentFlags().String("thermostat-id", "", "thermostat identifier")
	rootCmd.PersistentFlags().String("client-id", ecoclient.DefaultClientID, "Auth0 client ID")
	rootCmd.PersistentFlags().String("timezone", "", "IANA timezone for display")
	rootCmd.PersistentFlags().String("output", "table", "output format: table, json, csv")
	rootCmd.PersistentFlags().StringSlice("fields", nil, "comma-separated field filter")
	rootCmd.PersistentFlags().Bool("quiet", false, "suppress non-essential output")
	rootCmd.PersistentFlags().Bool("celsius", false, "display temperatures in Celsius")

	rootCmd.AddCommand(
		newStatusCmd(),
		newModeCmd(),
		newTempCmd(),
		newFanCmd(),
		newHoldCmd(),
		newResumeCmd(),
		newWeatherCmd(),
		newSensorsCmd(),
		newScheduleCmd(),
		newAlertsCmd(),
		newDevicesCmd(),
		newHomesCmd(),
		newEnergyCmd(),
		newWhoamiCmd(),
		newLogoutCmd(),
		newVersionCmd(),
		newCheckClientIDCmd(),
	)

	return rootCmd
}

func buildClient(cmd *cobra.Command, authRequired bool) (*ecoclient.Client, error) {
	cfg := state.loaded.Config
	cache := tokencache.New("ecobeectl", config.DefaultCacheDir())
	client := ecoclient.New(cfg.Email, cfg.Password, cfg.ThermostatID, cfg.ClientID, cache)
	client.Verbose = cfg.Verbose
	client.ClientIDSource = state.loaded.Sources["client_id"]
	if authRequired {
		if strings.TrimSpace(cfg.Email) == "" {
			return nil, fmt.Errorf("email is required; set email in ~/.config/ecobeectl/config.yaml, set ECOBEECTL_EMAIL, or pass --email")
		}
		needPassword, err := passwordRequired(client)
		if err != nil {
			return nil, err
		}
		if needPassword && cfg.Password == "" {
			password, err := promptPassword("Ecobee password: ")
			if err != nil {
				return nil, err
			}
			client.Password = password
		}
	}
	if cfg.Verbose {
		log.Debugf("using client_id from %s", clientIDSourceLabel(client.ClientIDSource))
	}
	return client, nil
}

func render(cmd *cobra.Command, headers []string, rows []map[string]any) error {
	format, err := output.ParseFormat(state.loaded.Config.Output)
	if err != nil {
		return err
	}
	headers, rows = output.FilterFields(headers, rows, state.loaded.Config.Fields)
	return output.Print(cmd.OutOrStdout(), format, headers, rows)
}

func nowInConfiguredLocation() *time.Location {
	if tz := state.loaded.Config.Timezone; tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	return time.Local
}

func passwordRequired(client *ecoclient.Client) (bool, error) {
	cached, err := client.Cache.Load(tokencache.Identity{
		AuthURL:  client.AuthURL,
		ClientID: client.ClientID,
		Email:    client.Email,
	})
	if err == nil {
		if cached.AccessToken != "" && time.Now().Before(cached.ExpiresAt) {
			return false, nil
		}
		if cached.RefreshToken != "" {
			return false, nil
		}
	}
	return true, nil
}

func clientIDSourceLabel(source string) string {
	switch source {
	case "flag":
		return "--client-id"
	case "env":
		return "ECOBEECTL_CLIENT_ID"
	case "config":
		return "~/.config/ecobeectl/config.yaml"
	default:
		return "built-in default"
	}
}

func contextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 45*time.Second)
}

func getStringFlag(cmd *cobra.Command, name string) string {
	value, _ := cmd.Flags().GetString(name)
	return value
}
