package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const EnvPrefix = "ECOBEECTL"

type Config struct {
	Email        string   `mapstructure:"email"`
	Password     string   `mapstructure:"password"`
	ThermostatID string   `mapstructure:"thermostat_id"`
	ClientID     string   `mapstructure:"client_id"`
	Timezone     string   `mapstructure:"timezone"`
	Output       string   `mapstructure:"output"`
	Fields       []string `mapstructure:"fields"`
	Verbose      bool     `mapstructure:"verbose"`
	UseCelsius   bool     `mapstructure:"use_celsius"`
	Quiet        bool     `mapstructure:"quiet"`
}

type Loaded struct {
	Config     Config
	ConfigFile string
	Sources    map[string]string
	Warnings   []string
}

func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(home, ".config", "ecobeectl", "config.yaml")
}

func DefaultCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".config", "ecobeectl", "keyring")
}

func Load(configPath string, flags *pflag.FlagSet) (Loaded, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()
	v.SetDefault("output", "table")
	if flags != nil {
		for _, binding := range []struct {
			key  string
			flag string
		}{
			{key: "email", flag: "email"},
			{key: "password", flag: "password"},
			{key: "thermostat_id", flag: "thermostat-id"},
			{key: "client_id", flag: "client-id"},
			{key: "timezone", flag: "timezone"},
			{key: "output", flag: "output"},
			{key: "fields", flag: "fields"},
			{key: "verbose", flag: "verbose"},
			{key: "use_celsius", flag: "celsius"},
			{key: "quiet", flag: "quiet"},
		} {
			if flag := flags.Lookup(binding.flag); flag != nil {
				if err := v.BindPFlag(binding.key, flag); err != nil {
					return Loaded{}, fmt.Errorf("bind flag %s: %w", binding.flag, err)
				}
			}
		}
	}

	if configPath == "" {
		configPath = DefaultConfigPath()
	}
	v.SetConfigFile(configPath)

	loaded := Loaded{
		ConfigFile: configPath,
		Sources:    map[string]string{},
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return Loaded{}, fmt.Errorf("read config: %w", err)
		}
	} else {
		loaded.ConfigFile = v.ConfigFileUsed()
		if warning := permissionWarning(loaded.ConfigFile); warning != "" {
			loaded.Warnings = append(loaded.Warnings, warning)
		}
	}

	if err := v.Unmarshal(&loaded.Config); err != nil {
		return Loaded{}, fmt.Errorf("decode config: %w", err)
	}
	loaded.Config.Fields = normalizeFields(loaded.Config.Fields)

	keys := []string{
		"email",
		"password",
		"thermostat_id",
		"client_id",
		"timezone",
		"output",
		"fields",
		"verbose",
		"use_celsius",
		"quiet",
	}
	for _, key := range keys {
		loaded.Sources[key] = detectSource(v, flags, key)
	}

	return loaded, nil
}

func EnvName(key string) string {
	return EnvPrefix + "_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
}

func detectSource(v *viper.Viper, flags *pflag.FlagSet, key string) string {
	flagName := strings.ReplaceAll(key, "_", "-")
	if key == "use_celsius" {
		flagName = "celsius"
	}
	if flags != nil {
		if flag := flags.Lookup(flagName); flag != nil && flag.Changed {
			return "flag"
		}
	}
	if _, ok := os.LookupEnv(EnvName(flagName)); ok {
		return "env"
	}
	if v.InConfig(key) {
		return "config"
	}
	return "default"
}

func normalizeFields(fields []string) []string {
	if len(fields) == 0 {
		return nil
	}
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		for _, part := range strings.Split(field, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func permissionWarning(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Sprintf("config file %s should be chmod 0600 because it may contain credentials", path)
	}
	return ""
}
