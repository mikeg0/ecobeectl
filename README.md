# 🌡️ ecobeectl - Control your thermostat, from the terminal

`ecobeectl` is a Go CLI for reading and controlling ecobee thermostats through the same cloud APIs used by the ecobee consumer portal.

## Quickstart

```bash
go build ./cmd/ecobeectl
./ecobeectl status --email you@example.com
```

The CLI supports config file, env, and flag-based configuration with this precedence:

1. CLI flags
2. `ECOBEECTL_*` environment variables
3. `~/.config/ecobeectl/config.yaml`
4. Built-in defaults

## Config

Example `~/.config/ecobeectl/config.yaml`:

```yaml
email: "you@example.com"
password: "your-password"
client_id: "183eORFPlXyz9BbDZwqexHPBQoVjgadh"
thermostat_id: "123456789012"
timezone: "America/Denver"
output: "table"
use_celsius: false
```

Restrict permissions because the file may contain credentials:

```bash
chmod 600 ~/.config/ecobeectl/config.yaml
```

## Global Flags

These flags are available on every command:

| Flag | Description |
|------|-------------|
| `--config <path>` | Config file path (default: `~/.config/ecobeectl/config.yaml`) |
| `-v, --verbose` | Enable debug logging |
| `--email <email>` | Ecobee account email |
| `--password <password>` | Ecobee account password |
| `--thermostat-id <id>` | Thermostat identifier |
| `--client-id <id>` | Auth0 client ID |
| `--timezone <tz>` | IANA timezone for display (e.g. `America/Denver`) |
| `--output <format>` | Output format: `table`, `json`, `csv` (default: `table`) |
| `--fields <fields>` | Comma-separated field filter |
| `--quiet` | Suppress non-essential output |
| `--celsius` | Display temperatures in Celsius |

## Commands

### Control

#### `status` - Show thermostat status

```bash
ecobeectl status
```

When a hold is active, the output adds `hold_type`, `hold_heat`, `hold_cool`, and `hold_ends` columns showing the held setpoints and when the hold expires (`indefinite` if it has no end).

#### `mode` - Set HVAC mode

```bash
ecobeectl mode <heat|cool|auto|off>
```

#### `temp` - Set temperature hold

```bash
# Heat or cool mode — single value
ecobeectl temp 68

# Auto mode — both setpoints required
ecobeectl temp --heat 67 --cool 75
```

| Flag | Description |
|------|-------------|
| `--heat <value>` | Heat setpoint (required in auto mode) |
| `--cool <value>` | Cool setpoint (required in auto mode) |
| `--hold-type <type>` | Hold type (default: `indefinite`) |

#### `fan` - Set fan mode

```bash
ecobeectl fan <on|auto>
```

| Flag | Description |
|------|-------------|
| `--hold-type <type>` | Hold type (default: `nextTransition`) |

#### `hold` - Set climate hold

```bash
ecobeectl hold <climate>
```

`<climate>` is a climate reference name (e.g. `home`, `away`, `sleep`).

| Flag | Description |
|------|-------------|
| `--hold-type <type>` | Hold type (default: `indefinite`) |

#### `resume` - Resume scheduled program

```bash
ecobeectl resume
```

### Read

#### `weather` - Show weather forecast

```bash
ecobeectl weather
```

#### `sensors` - Show remote sensor readings

```bash
ecobeectl sensors
```

#### `schedule` - Show program schedule

```bash
ecobeectl schedule
```

#### `events` - Show thermostat events

```bash
ecobeectl events
```

Lists every event ecobee has stacked on the thermostat — manual holds, vacations, occupancy automation, and eco+ Time of Use precool/setback events — with the `type`, `name`, `running` flag, setpoints, and start/end times. Use it to see what actually set the active hold reported by `status` (e.g. a `touPrecool` event named `prc150000`), since `status` only shows the highest-priority running event.

#### `alerts` - Show thermostat alerts

```bash
ecobeectl alerts
```

#### `devices` - List registered devices

```bash
ecobeectl devices
```

#### `homes` - Show home structure

```bash
ecobeectl homes
```

#### `energy` - Show Energy IQ data

```bash
ecobeectl energy --start 2026-03-01 --end 2026-03-14
```

Defaults to the last 7 days if `--start` and `--end` are omitted.

| Flag | Description |
|------|-------------|
| `--start <date>` | Report start date (`YYYY-MM-DD`) |
| `--end <date>` | Report end date (`YYYY-MM-DD`) |

#### `air-quality` - Show air quality over time

```bash
ecobeectl air-quality --start 2026-04-16 --end 2026-04-23
```

Pulls the ecobee runtime report at 5-minute intervals and returns the `air_quality` score, `accuracy`, `co2_ppm`, `voc_ppb`, and `air_pressure` columns. These readings come directly from the thermostat's own air quality sensor (not a weather service). Defaults to the last 7 days if `--start` and `--end` are omitted. Thermostats without an air quality sensor will return empty values.

> **Note on units:** CO₂ is reported in parts per million (`co2_ppm`). VOC values come from the ecobee API's `vocPPM` capability, but the underlying readings are actually in parts per **billion**, so the column is labelled `voc_ppb`. The `air_quality` score is a relative 0–100 index (higher is cleaner), not a concentration.

> **Note on dates:** `--start`/`--end` are treated as local-time dates. The ecobee runtime report interprets its date range as UTC but stamps each row in the thermostat's local time, so `ecobeectl` requests an extra day on each side and trims the result back to the dates you asked for. Without this you'd see rows from the day before `--start` (and miss the end of `--end`).

| Flag | Description |
|------|-------------|
| `--start <date>` | Report start date (`YYYY-MM-DD`) |
| `--end <date>` | Report end date (`YYYY-MM-DD`) |

### Account

#### `whoami` - Show user account information

```bash
ecobeectl whoami
```

#### `logout` - Clear cached tokens

```bash
ecobeectl logout --email you@example.com
```

Requires `--email` so the correct cached identity can be selected.

#### `version` - Print version information

```bash
ecobeectl version
```

#### `check-client-id` - Compare configured client ID with the ecobee website

```bash
ecobeectl check-client-id
```

Reports whether the effective client ID matches the one currently used by the ecobee website.

## Client ID Overrides

The portal client ID is baked in as the default, but it can be overridden if ecobee rotates it:

- `client_id:` in `~/.config/ecobeectl/config.yaml`
- `ECOBEECTL_CLIENT_ID`
- `--client-id`

If auth fails with an invalid or unauthorized client error, run `ecobeectl check-client-id` and update the configured client ID if needed.

## Development

```bash
make fmt
make test
make build
```

Optional live integration test:

```bash
ECOBEECTL_LIVE_EMAIL=you@example.com \
ECOBEECTL_LIVE_PASSWORD=secret \
ECOBEECTL_LIVE_THERMOSTAT_ID=123456789012 \
go test ./internal/client -run TestLiveStatus
```
