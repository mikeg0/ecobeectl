package cmd

import (
	"github.com/spf13/cobra"

	ecoclient "github.com/mikeg/ecobeectl/internal/client"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			row := map[string]any{
				"version": version,
				"commit":  commit,
				"date":    date,
			}
			return render(cmd, []string{"version", "commit", "date"}, []map[string]any{row})
		},
	}
}

func newCheckClientIDCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check-client-id",
		Short: "Compare the configured client ID with the current ecobee website value",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := state.loaded.Config
			detector := ecoclient.ClientIDDetector{}
			ctx, cancel := contextWithTimeout()
			defer cancel()
			detectedID, source, err := detector.Detect(ctx)
			status := "match"
			if err != nil {
				status = "unable to detect"
			} else if detectedID != cfg.ClientID {
				status = "mismatch"
			}
			row := map[string]any{
				"effective_client_id": cfg.ClientID,
				"effective_source":    clientIDSourceLabel(state.loaded.Sources["client_id"]),
				"default_client_id":   ecoclient.DefaultClientID,
				"detected_client_id":  detectedID,
				"detected_source":     source,
				"status":              status,
			}
			if err != nil && cfg.Verbose {
				row["error"] = err.Error()
			}
			if status == "mismatch" && !cfg.Verbose {
				row["hint"] = "set client_id in config, set ECOBEECTL_CLIENT_ID, or pass --client-id"
			}
			return render(cmd, []string{"effective_client_id", "effective_source", "default_client_id", "detected_client_id", "detected_source", "status", "hint", "error"}, []map[string]any{row})
		},
	}
}
