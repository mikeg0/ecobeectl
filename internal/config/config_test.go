package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
)

func TestLoadPrecedenceAndSources(t *testing.T) {
	t.Setenv("ECOBEECTL_EMAIL", "env@example.com")
	t.Setenv("ECOBEECTL_PASSWORD", "env-password")
	t.Setenv("ECOBEECTL_CLIENT_ID", "env-client")
	t.Setenv("ECOBEECTL_USE_CELSIUS", "true")

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("email: config@example.com\npassword: config-password\nclient_id: config-client\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("email", "", "")
	flags.String("password", "", "")
	flags.String("thermostat-id", "", "")
	flags.String("client-id", DefaultConfigPath(), "")
	flags.String("timezone", "", "")
	flags.String("output", "table", "")
	flags.StringSlice("fields", nil, "")
	flags.Bool("verbose", false, "")
	flags.Bool("celsius", false, "")
	flags.Bool("quiet", false, "")
	if err := flags.Set("email", "flag@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := flags.Set("client-id", "flag-client"); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(configPath, flags)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := loaded.Config.Email, "flag@example.com"; got != want {
		t.Fatalf("email = %q, want %q", got, want)
	}
	if got, want := loaded.Config.Password, "env-password"; got != want {
		t.Fatalf("password = %q, want %q", got, want)
	}
	if got, want := loaded.Config.ClientID, "flag-client"; got != want {
		t.Fatalf("client_id = %q, want %q", got, want)
	}
	if !loaded.Config.UseCelsius {
		t.Fatalf("use_celsius should be true from env")
	}
	if got, want := loaded.Sources["email"], "flag"; got != want {
		t.Fatalf("email source = %q, want %q", got, want)
	}
	if got, want := loaded.Sources["password"], "env"; got != want {
		t.Fatalf("password source = %q, want %q", got, want)
	}
	if got, want := loaded.Sources["client_id"], "flag"; got != want {
		t.Fatalf("client_id source = %q, want %q", got, want)
	}
}

func TestLoadPermissionWarning(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("email: test@example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("email", "", "")
	flags.String("password", "", "")
	flags.String("thermostat-id", "", "")
	flags.String("client-id", "", "")
	flags.String("timezone", "", "")
	flags.String("output", "table", "")
	flags.StringSlice("fields", nil, "")
	flags.Bool("verbose", false, "")
	flags.Bool("celsius", false, "")
	flags.Bool("quiet", false, "")

	loaded, err := Load(configPath, flags)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Warnings) == 0 {
		t.Fatalf("expected a permission warning")
	}
}
