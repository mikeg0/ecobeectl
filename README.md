# ecobeectl

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
