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

## Commands

```bash
ecobeectl status
ecobeectl mode heat
ecobeectl temp 68
ecobeectl temp --heat 67 --cool 75
ecobeectl fan on
ecobeectl hold home
ecobeectl resume
ecobeectl weather
ecobeectl sensors
ecobeectl schedule
ecobeectl alerts
ecobeectl devices
ecobeectl homes
ecobeectl energy --start 2026-03-01 --end 2026-03-14
ecobeectl whoami
ecobeectl logout --email you@example.com
ecobeectl check-client-id
```

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
