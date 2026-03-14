# Project Plan: `ecobeectl` — A Go CLI for Controlling ecobee Thermostats

## 1. Project Overview

`ecobeectl` is a Go CLI tool that talks to the same undocumented cloud endpoints the ecobee consumer web portal uses. It authenticates via Auth0 Resource Owner Password Grant against `auth.ecobee.com`, caches JWT tokens locally, and provides subcommands for reading thermostat state, changing temperature setpoints, switching HVAC modes, controlling fan and hold behaviors, viewing weather, sensors, schedules, and more.

The project structure, configuration system, token caching strategy, output formatting, and CI/release pipeline are modeled directly on [steipete/eightctl](https://github.com/steipete/eightctl).

---

## 2. Discovered API Surface

All data below was captured from a live authenticated browser session against the ecobee consumer web portal.

### 2.1 Authentication

**Provider:** Auth0 (hosted at `auth.ecobee.com`)

**Token endpoint:** `POST https://auth.ecobee.com/oauth/token`

**Request body:**
```json
{
  "grant_type": "password",
  "client_id": "183eORFPlXyz9BbDZwqexHPBQoVjgadh",
  "username": "<email>",
  "password": "<password>",
  "audience": "https://prod.ecobee.com/api/v1",
  "scope": "openid smartWrite piiWrite piiRead smartRead deleteGrants offline_access"
}
```

**Expected response:**
```json
{
  "access_token": "<JWT>",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "<refresh_token>"  // if offline_access scope is granted
}
```

**Key facts:**
- The JWT is issued by `https://auth.ecobee.com/`, audience `https://prod.ecobee.com/api/v1`.
- Token lifetime is **1 hour**.
- JWT payload keys: `iss`, `sub`, `aud`, `iat`, `exp`, `scope`, `azp`, `https://claims.ecobee.com/ecobee_account_id`.
- Scopes granted: `openid`, `piiRead`, `piiWrite`, `smartRead`, `smartWrite`, `deleteGrants`.
- The `offline_access` scope was not rejected in testing, suggesting refresh tokens are available.
- The web portal stores the token in a `_TOKEN` cookie.
- The client ID `183eORFPlXyz9BbDZwqexHPBQoVjgadh` is the public web app client credential (extracted from the authorize URL on ecobee.com).

**Refresh flow** (standard Auth0):
```json
POST https://auth.ecobee.com/oauth/token
{
  "grant_type": "refresh_token",
  "client_id": "183eORFPlXyz9BbDZwqexHPBQoVjgadh",
  "refresh_token": "<refresh_token>"
}
```

### 2.2 API Endpoints

**Base URL:** `https://api.ecobee.com`

All REST endpoints require `Authorization: Bearer <token>` header and use `Content-Type: application/json`. GET requests pass structured query data as a JSON-encoded URL parameter. All REST endpoints include `?format=json`.

#### 2.2.1 `GET /1/thermostat` — Read full thermostat state

**Query parameters:**
```
?format=json&json=<URL-encoded JSON>&_timestamp=<epoch_ms>
```

**JSON parameter structure:**
```json
{
  "selection": {
    "selectionType": "thermostats",
    "selectionMatch": "<thermostat_id>",
    "includeEvents": true,
    "includeProgram": true,
    "includeSettings": true,
    "includeRuntime": true,
    "includeAlerts": true,
    "includeWeather": true,
    "includeExtendedRuntime": true,
    "includeLocation": true,
    "includeHouseDetails": true,
    "includeNotificationSettings": true,
    "includeTechnician": true,
    "includePrivacy": true,
    "includeVersion": true,
    "includeOemCfg": true,
    "includeSecuritySettings": true,
    "includeSensors": true,
    "includeUtility": true,
    "includeAudio": true
  }
}
```

**Response top-level keys:** `page`, `thermostatList`, `status`

**`thermostatList[0]` keys:** `identifier`, `name`, `thermostatRev`, `isRegistered`, `modelNumber`, `brand`, `features`, `lastModified`, `thermostatTime`, `utcTime`, `audio`, `alerts`, `settings`, `runtime`, `extendedRuntime`, `location`, `technician`, `utility`, `weather`, `events`, `program`, `houseDetails`, `oemCfg`, `notificationSettings`, `privacy`, `version`, `securitySettings`, `remoteSensors`

**`runtime` keys:** `runtimeRev`, `connected`, `firstConnected`, `connectDateTime`, `disconnectDateTime`, `lastModified`, `lastStatusModified`, `runtimeDate`, `runtimeInterval`, `actualTemperature`, `actualHumidity`, `rawTemperature`, `showIconMode`, `desiredHeat`, `desiredCool`, `desiredHumidity`, `desiredDehumidity`, `desiredFanMode`, `actualVOC`, `actualCO2`, `actualAQAccuracy`, `actualAQScore`, `desiredHeatRange`, `desiredCoolRange`

Note: temperatures are in **tenths of degrees Fahrenheit** (e.g., `730` = 73.0°F).

**`settings` keys (partial, most important):** `hvacMode`, `fanMinOnTime`, `holdAction`, `heatCoolMinDelta`, `tempCorrection`, `coldTempAlert`, `coldTempAlertEnabled`, `hotTempAlert`, `hotTempAlertEnabled`, `coolStages`, `heatStages`, `hasHeatPump`, `hasForcedAir`, `hasHumidifier`, `hasDehumidifier`, `dehumidifierMode`, `dehumidifierLevel`, `humidity`, `humidifierMode`, `useCelsius`, `locale`, `backlightOnIntensity`, `backlightSleepIntensity`, `compressorProtectionMinTime`, `stage1HeatingDifferentialTemp`, `stage1CoolingDifferentialTemp`, and dozens more.

**`events[0]` keys:** `type`, `name`, `running`, `startDate`, `startTime`, `endDate`, `endTime`, `isOccupied`, `isCoolOff`, `isHeatOff`, `coolHoldTemp`, `heatHoldTemp`, `fan`, `vent`, `ventilatorMinOnTime`, `isOptional`, `isTemperatureRelative`, `coolRelativeTemp`, `heatRelativeTemp`, `isTemperatureAbsolute`, `dutyCyclePercentage`, `fanMinOnTime`, `occupiedSensorActive`, `unoccupiedSensorActive`, `drRampUpTemp`, `drRampUpTime`, `linkRef`, `holdClimateRef`, `fanSpeed`, `isIndefinite`

**`program` keys:** `schedule`, `climates`, `currentClimateRef`

**`program.climates[0]` keys:** `name`, `climateRef`, `isOccupied`, `isOptimized`, `coolFan`, `heatFan`, `vent`, `ventilatorMinOnTime`, `owner`, `type`, `colour`, `coolTemp`, `heatTemp`, `sensors`

**`remoteSensors[0]` keys:** `id`, `name`, `type`, `inUse`, `capability`
**`remoteSensors[0].capability[0]` keys:** `id`, `type`, `value`

**`weather.forecasts[0]` keys:** `weatherSymbol`, `dateTime`, `condition`, `temperature`, `pressure`, `relativeHumidity`, `dewpoint`, `visibility`, `windSpeed`, `windGust`, `windDirection`, `windBearing`, `pop`, `tempHigh`, `tempLow`, `sky`

#### 2.2.2 `POST /1/thermostat` — Update thermostat

**Query parameters:** `?format=json`

**Two body patterns observed:**

**Pattern A — Settings update (e.g., HVAC mode):**
```json
{
  "selection": {
    "selectionType": "thermostats",
    "selectionMatch": "<thermostat_id>"
  },
  "thermostat": {
    "settings": {
      "hvacMode": "heat"
    }
  }
}
```
Valid `hvacMode` values observed: `"heat"`, `"cool"`, `"auto"`, `"off"`

**Pattern B — Function call (holds, fan, resume):**
```json
{
  "selection": {
    "selectionType": "thermostats",
    "selectionMatch": "<thermostat_id>"
  },
  "functions": [{
    "type": "<function_type>",
    "params": { ... }
  }]
}
```

**Observed function types and params:**

`setHold` — temperature:
```json
{"type": "setHold", "params": {"coolHoldTemp": 730, "heatHoldTemp": 730, "holdType": "indefinite"}}
```

`setHold` — fan:
```json
{"type": "setHold", "params": {"coolHoldTemp": 920, "heatHoldTemp": 450, "holdType": "indefinite", "fan": "on", "isTemperatureAbsolute": false, "isTemperatureRelative": false}}
```
Fan values: `"on"`, `"auto"`

`setHold` — climate ref (Home/Away):
```json
{"type": "setHold", "params": {"holdType": "indefinite", "holdClimateRef": "home"}}
```
Climate ref values: `"home"`, `"away"`

`resumeProgram` — cancel hold:
```json
{"type": "resumeProgram"}
```

**Response:** `{"status": {"code": 0, "message": ""}}` (code 0 = success)

#### 2.2.3 `GET /1/thermostatSummary` — Quick status poll

**Query parameters:**
```
?format=json&json={"selection":{"selectionType":"thermostats","selectionMatch":"","includeEquipmentStatus":true}}&_timestamp=<epoch_ms>
```

**Response keys:** `thermostatCount`, `revisionList`, `statusList`, `status`

#### 2.2.4 `GET /1/energyIqReport` — Energy usage

**Query parameters:**
```
?format=json&body={"selection":{"selectionType":"thermostats","selectionMatch":"<id>","includeAlerts":true},"startDate":"YYYY-MM-DD","endDate":"YYYY-MM-DD"}&_timestamp=<epoch_ms>
```

**Response keys:** `data`, `status`

#### 2.2.5 `GET /1/user` — User info

**Response keys:** `user`, `status`

#### 2.2.6 `GET /1/group` — Group info

**Response keys:** `groups`, `status`

#### 2.2.7 `GET /ea/devices/ls` — Device listing

**Query parameters:** `?format=json`

**Response keys:** `count`, `devices`

#### 2.2.8 `POST /graphql` → `beehive.ecobee.com` — Home structure

**URL:** `https://beehive.ecobee.com/graphql`

**Body:**
```json
{
  "query": "query SPHomesQuery { homes { id name permissions members { id role } devices { thermostats { id } lightSwitches { id } } } unassigned { thermostats { id } lightSwitches { id } } }"
}
```

**Response keys:** `data` (containing `homes` and `unassigned`)

---

## 3. Project Structure

Mirroring eightctl's layout:

```
ecobeectl/
├── .github/
│   └── workflows/
│       └── ci.yml                  # Format check, lint, test
├── cmd/
│   └── ecobeectl/
│       └── main.go                 # Entry point: calls cmd.Execute()
├── internal/
│   ├── client/
│   │   ├── ecobee.go              # Core HTTP client, auth, token management, do() helper
│   │   ├── ecobee_test.go         # Client unit tests (mock HTTP)
│   │   ├── thermostat.go          # GET/POST /1/thermostat methods + types
│   │   ├── summary.go             # GET /1/thermostatSummary
│   │   ├── energy.go              # GET /1/energyIqReport
│   │   ├── devices.go             # GET /ea/devices/ls
│   │   ├── user.go                # GET /1/user
│   │   ├── group.go               # GET /1/group
│   │   └── homes.go               # POST /graphql (beehive SPHomesQuery)
│   ├── cmd/
│   │   ├── root.go                # Cobra root command, persistent flags, config init
│   │   ├── status.go              # `status` subcommand
│   │   ├── set_mode.go            # `mode` subcommand (heat/cool/auto/off)
│   │   ├── set_temp.go            # `temp` subcommand
│   │   ├── fan.go                 # `fan` subcommand (on/auto)
│   │   ├── hold.go                # `hold` subcommand (home/away)
│   │   ├── resume.go              # `resume` subcommand
│   │   ├── weather.go             # `weather` subcommand
│   │   ├── sensors.go             # `sensors` subcommand
│   │   ├── schedule.go            # `schedule` subcommand
│   │   ├── alerts.go              # `alerts` subcommand
│   │   ├── devices.go             # `devices` subcommand
│   │   ├── homes.go               # `homes` subcommand
│   │   ├── energy.go              # `energy` subcommand
│   │   ├── whoami.go              # `whoami` subcommand
│   │   ├── logout.go              # `logout` subcommand (clear cached token)
│   │   ├── version.go             # `version` subcommand
│   │   └── util.go                # Shared helpers (mapKeys, temperature conversion, etc.)
│   ├── config/
│   │   └── config.go              # Viper-based config loading (YAML + env + flags)
│   ├── output/
│   │   └── output.go              # Table/JSON/CSV output formatter
│   └── tokencache/
│       └── tokencache.go          # Keyring-backed token + refresh token caching
├── .gitignore
├── .golangci.yml                   # Linter config
├── .goreleaser.yaml                # Cross-platform release builds
├── LICENSE                         # MIT
├── Makefile                        # fmt, lint, test targets
├── README.md
├── go.mod
└── go.sum
```

---

## 4. Detailed Component Specifications

### 4.1 `internal/config/config.go` — Configuration

Follows eightctl's pattern exactly: Viper-based with YAML config file, env vars, and CLI flags.

**Config struct:**
```go
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
}
```

**Priority:** CLI flags > env vars (`ECOBEECTL_*`) > config file (`~/.config/ecobeectl/config.yaml`)

**Config file example:**
```yaml
email: "you@example.com"
password: "your-password"
# thermostat_id: "531668456552"  # optional; auto-resolved if you have one thermostat
# timezone: "America/Denver"      # defaults to local
# use_celsius: false              # defaults to false
```

Permission check: warn if config file permissions are more open than `0600`.

### 4.2 `internal/tokencache/tokencache.go` — Token Caching

Uses `github.com/99designs/keyring` (same as eightctl) for secure, cross-platform token storage. Supports macOS Keychain, Linux SecretService/libsecret, Windows Credential Manager, with file-based fallback.

**CachedToken struct:**
```go
type CachedToken struct {
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token,omitempty"`
    ExpiresAt    time.Time `json:"expires_at"`
    AccountID    string    `json:"account_id,omitempty"`
}
```

**Identity struct** (for namespacing cached tokens):
```go
type Identity struct {
    AuthURL  string  // "https://auth.ecobee.com"
    ClientID string
    Email    string
}
```

**Functions:** `Save(id, token)`, `Load(id) (*CachedToken, error)`, `Clear(id) error`

Cache key format: `ecobeectl-token:<auth_url>|<client_id>|<email>`

Storage location: system keyring, with fallback to `~/.config/ecobeectl/keyring/`

### 4.3 `internal/client/ecobee.go` — Core Client

**Constants:**
```go
const (
    defaultAPIBaseURL  = "https://api.ecobee.com"
    defaultAuthURL     = "https://auth.ecobee.com/oauth/token"
    defaultGraphQLURL  = "https://beehive.ecobee.com/graphql"
    defaultClientID    = "183eORFPlXyz9BbDZwqexHPBQoVjgadh"
    defaultAudience    = "https://prod.ecobee.com/api/v1"
    defaultScopes      = "openid smartWrite piiWrite piiRead smartRead deleteGrants offline_access"
)
```

**Client struct:**
```go
type Client struct {
    Email        string
    Password     string
    ClientID     string
    ThermostatID string
    HTTP         *http.Client
    APIBaseURL   string
    AuthURL      string
    GraphQLURL   string
    token        string
    refreshToken string
    tokenExp     time.Time
}
```

**`New(email, password, thermostatID, clientID string) *Client`** — Constructor with defaults.

**Authentication flow (`Authenticate(ctx) error`):**

1. `POST https://auth.ecobee.com/oauth/token` with `grant_type=password`, email, password, client_id, audience, scope.
2. Parse response for `access_token`, `expires_in`, `refresh_token`.
3. Set `c.token`, `c.refreshToken`, `c.tokenExp = now + expires_in - 60s` (1-minute buffer).
4. Cache both tokens via `tokencache.Save()`.

**Token refresh flow (`refreshAuth(ctx) error`):**

1. `POST https://auth.ecobee.com/oauth/token` with `grant_type=refresh_token`, client_id, refresh_token.
2. Parse new access_token (and possibly new refresh_token).
3. Update in-memory state and cache.

**Token resolution (`ensureToken(ctx) error`)** — mirrors eightctl:

1. If in-memory token is valid, use it.
2. Try loading from keyring cache. If valid, use it.
3. If a refresh token exists and the access token is expired, call `refreshAuth()`.
4. Otherwise, call `Authenticate()` (full username/password login).

**`do(ctx, method, path, query, body, out) error`** — Generic HTTP helper:

1. Call `ensureToken(ctx)`.
2. Build request URL: `APIBaseURL + path + "?" + query.Encode()`.
3. Set headers: `Authorization: Bearer <token>`, `Content-Type: application/json`, `Accept: application/json`.
4. Execute request.
5. On **401 Unauthorized**: clear cached token, call `ensureToken()` again (triggers refresh or re-auth), retry once.
6. On **429 Too Many Requests**: sleep 2s and retry.
7. On success: decode JSON response into `out`.

**`doGraphQL(ctx, query string, out any) error`** — GraphQL helper targeting `beehive.ecobee.com`.

**`EnsureThermostatID(ctx) error`** — If `ThermostatID` is empty, call `GET /ea/devices/ls` or `GET /1/thermostatSummary` to auto-detect. If exactly one thermostat, use it. If multiple, return an error listing them.

### 4.4 `internal/client/thermostat.go` — Thermostat Operations

**Types:**
```go
type Thermostat struct {
    Identifier    string          `json:"identifier"`
    Name          string          `json:"name"`
    ModelNumber   string          `json:"modelNumber"`
    ThermostatRev string          `json:"thermostatRev"`
    ThermostatTime string         `json:"thermostatTime"`
    UTCTime       string          `json:"utcTime"`
    Runtime       Runtime         `json:"runtime"`
    Settings      Settings        `json:"settings"`
    Events        []Event         `json:"events"`
    Program       Program         `json:"program"`
    Weather       Weather         `json:"weather"`
    RemoteSensors []RemoteSensor  `json:"remoteSensors"`
    Alerts        []Alert         `json:"alerts"`
    // ... other fields as needed
}

type Runtime struct {
    Connected       bool   `json:"connected"`
    ActualTemperature int  `json:"actualTemperature"`  // tenths of °F
    ActualHumidity  int    `json:"actualHumidity"`
    DesiredHeat     int    `json:"desiredHeat"`
    DesiredCool     int    `json:"desiredCool"`
    DesiredFanMode  string `json:"desiredFanMode"`
    ActualVOC       int    `json:"actualVOC"`
    ActualCO2       int    `json:"actualCO2"`
    ActualAQScore   int    `json:"actualAQScore"`
}

type Settings struct {
    HvacMode          string `json:"hvacMode"`
    FanMinOnTime      int    `json:"fanMinOnTime"`
    HeatCoolMinDelta  int    `json:"heatCoolMinDelta"`
    CoolStages        int    `json:"coolStages"`
    HeatStages        int    `json:"heatStages"`
    UseCelsius        bool   `json:"useCelsius"`
    Humidity          string `json:"humidity"`
    HoldAction        string `json:"holdAction"`
    // ... extensive, include all captured fields
}

type Event struct {
    Type             string `json:"type"`
    Name             string `json:"name"`
    Running          bool   `json:"running"`
    HoldClimateRef   string `json:"holdClimateRef"`
    CoolHoldTemp     int    `json:"coolHoldTemp"`
    HeatHoldTemp     int    `json:"heatHoldTemp"`
    Fan              string `json:"fan"`
    HoldType         string `json:"holdType"`  // "indefinite", etc.
    IsIndefinite     bool   `json:"isIndefinite"`
    StartDate        string `json:"startDate"`
    EndDate          string `json:"endDate"`
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
```

**Methods:**

`GetThermostat(ctx, includes ...string) (*Thermostat, error)` — `GET /1/thermostat` with configurable include flags.

`SetHvacMode(ctx, mode string) error` — `POST /1/thermostat` with settings pattern. Validates mode is one of: `heat`, `cool`, `auto`, `off`.

`SetTemperatureHold(ctx, heatTemp, coolTemp int, holdType string) error` — `POST /1/thermostat` with `setHold` function. Temps in tenths of °F.

`SetFan(ctx, fanMode string) error` — `POST /1/thermostat` with `setHold` function including `fan` param. Requires reading current hold temps to preserve them.

`SetClimateHold(ctx, climateRef, holdType string) error` — `POST /1/thermostat` with `setHold` + `holdClimateRef`. Values: `"home"`, `"away"`, `"sleep"`.

`ResumeProgram(ctx) error` — `POST /1/thermostat` with `resumeProgram` function.

**Temperature helpers:**
```go
func FtoTenths(f float64) int          // 73.0 → 730
func TenthsToF(t int) float64          // 730 → 73.0
func TenthsToC(t int) float64          // 730 → 22.8
func CtoTenths(c float64) int          // 22.8 → 730
func ParseTemp(s string) (int, error)  // "73", "73F", "22.8C" → tenths of °F
```

### 4.5 `internal/client/summary.go`

`GetThermostatSummary(ctx) (*ThermostatSummary, error)` — Lightweight status poll.

### 4.6 `internal/client/energy.go`

`GetEnergyReport(ctx, startDate, endDate string) (*EnergyReport, error)` — Energy IQ data.

### 4.7 `internal/client/devices.go`

`ListDevices(ctx) ([]Device, error)` — `GET /ea/devices/ls`

### 4.8 `internal/client/user.go`

`GetUser(ctx) (*User, error)` — `GET /1/user`

### 4.9 `internal/client/homes.go`

`GetHomes(ctx) (*HomesResponse, error)` — GraphQL `SPHomesQuery` against `beehive.ecobee.com`.

### 4.10 `internal/output/output.go`

Identical to eightctl: `Print(format Format, headers []string, rows []map[string]any) error` with `table`, `json`, `csv` support. Plus `FilterFields()` for `--fields` filtering.

### 4.11 `internal/cmd/root.go`

**Persistent flags:**
- `--config` — config file path
- `--verbose` / `-v` — debug logging
- `--email` — ecobee account email
- `--password` — ecobee account password
- `--thermostat-id` — thermostat identifier (auto-resolved if only one)
- `--client-id` — Auth0 client ID (defaults to baked-in web app credential)
- `--timezone` — IANA timezone
- `--output` — table/json/csv
- `--fields` — output field filter
- `--quiet` — suppress config banner
- `--celsius` — display temperatures in Celsius

**Env var prefix:** `ECOBEECTL_` (e.g., `ECOBEECTL_EMAIL`, `ECOBEECTL_PASSWORD`)

**`requireAuthFields()`** — Same pattern as eightctl: check cache first, then require email+password.

**`newClient()`** — Shared helper to construct a `client.Client` from viper values. Used by all subcommands.

---

## 5. Command Surface

| Command | Description | API Call |
|---------|-------------|----------|
| `ecobeectl status` | Show current temp, humidity, mode, setpoints, fan, hold status, air quality | `GET /1/thermostat` |
| `ecobeectl mode <heat\|cool\|auto\|off>` | Set HVAC mode | `POST /1/thermostat` (settings) |
| `ecobeectl temp <value>` | Set temperature hold (e.g., `73`, `73F`, `22.8C`) | `POST /1/thermostat` (setHold) |
| `ecobeectl fan <on\|auto>` | Set fan mode | `POST /1/thermostat` (setHold) |
| `ecobeectl hold <home\|away\|sleep>` | Set climate hold | `POST /1/thermostat` (setHold) |
| `ecobeectl resume` | Cancel hold, resume schedule | `POST /1/thermostat` (resumeProgram) |
| `ecobeectl weather` | Show current weather + forecast | `GET /1/thermostat` (weather) |
| `ecobeectl sensors` | Show remote sensor readings | `GET /1/thermostat` (sensors) |
| `ecobeectl schedule` | Show current program schedule | `GET /1/thermostat` (program) |
| `ecobeectl alerts` | Show active alerts/reminders | `GET /1/thermostat` (alerts) |
| `ecobeectl devices` | List registered devices | `GET /ea/devices/ls` |
| `ecobeectl homes` | Show home structure | `POST /graphql` (SPHomesQuery) |
| `ecobeectl energy [--start DATE] [--end DATE]` | Energy usage report | `GET /1/energyIqReport` |
| `ecobeectl whoami` | Show user account info | `GET /1/user` |
| `ecobeectl logout` | Clear cached tokens | `tokencache.Clear()` |
| `ecobeectl version` | Print version | (local) |

All commands support `--output table|json|csv` and `--fields field1,field2`.

---

## 6. Dependencies

| Dependency | Purpose | Same as eightctl? |
|---|---|---|
| `github.com/spf13/cobra` | CLI framework | Yes |
| `github.com/spf13/viper` | Config management (YAML/env/flags) | Yes |
| `github.com/99designs/keyring` | Secure token caching (Keychain/SecretService/WinCred) | Yes |
| `github.com/charmbracelet/log` | Structured logging | Yes |
| `gopkg.in/yaml.v3` | YAML parsing | Yes |

---

## 7. Build & CI

### 7.1 Makefile
```makefile
.PHONY: build fmt lint test install

build:
	go build -o bin/ecobeectl ./cmd/ecobeectl

install:
	go install ./cmd/ecobeectl

fmt:
	gofumpt -w ./

lint:
	golangci-lint run ./...

test:
	go test ./...
```

### 7.2 CI (`.github/workflows/ci.yml`)

Same as eightctl: runs on push/PR to main. Steps: checkout, setup Go 1.24+, cache modules, install gofumpt + golangci-lint, format check, lint, test.

### 7.3 GoReleaser (`.goreleaser.yaml`)

Cross-compile for darwin/linux/windows (amd64 + arm64). `CGO_ENABLED=0` for static binaries. Tar.gz for unix, zip for Windows.

---

## 8. Implementation Phases

**Phase 1 — Foundation (auth + status)**
- Scaffold project structure (go.mod, cmd/, internal/)
- Implement `internal/config` (Viper config loading)
- Implement `internal/tokencache` (keyring-backed caching)
- Implement `internal/client/ecobee.go` (core client with auth, token refresh, do() helper)
- Implement `internal/output` (table/json/csv)
- Implement `status` command (GET /1/thermostat → display current state)
- Implement `whoami` command (GET /1/user)
- Implement `logout` command (clear cache)
- Implement `version` command
- Add root.go with all persistent flags and config init
- **Milestone: `ecobeectl status` works end-to-end with username/password auth**

**Phase 2 — Core thermostat control**
- Implement `mode` command (set hvacMode)
- Implement `temp` command (setHold with temperature, including °F/°C parsing)
- Implement `fan` command (setHold with fan param)
- Implement `hold` command (setHold with holdClimateRef)
- Implement `resume` command (resumeProgram)
- Add temperature conversion helpers
- **Milestone: full thermostat control from CLI**

**Phase 3 — Read-only data commands**
- Implement `weather` command
- Implement `sensors` command
- Implement `schedule` command
- Implement `alerts` command
- Implement `devices` command (GET /ea/devices/ls)
- Implement `homes` command (GraphQL SPHomesQuery)
- Implement `energy` command (energy IQ report)
- **Milestone: complete read access to all thermostat data**

**Phase 4 — Polish & release**
- Add unit tests (mock HTTP client for each endpoint)
- Add integration test scaffolding (optional, needs real credentials)
- Set up .golangci.yml, Makefile, CI workflow
- Set up .goreleaser.yaml
- Write comprehensive README with quickstart, command reference, config examples
- Tag v0.1.0 release
- **Milestone: CI-green, cross-platform release artifacts**

---

## 9. Key Design Decisions

**Temperature unit handling:** Internally always work in ecobee's native format (tenths of °F). Convert to/from °F or °C only at the CLI boundary based on `--celsius` flag or `use_celsius` config.

**Thermostat ID auto-resolution:** If not configured, call `GET /ea/devices/ls` on first use. If exactly one thermostat, cache the ID in memory for the session. If multiple, print a list and ask the user to set `thermostat_id` in config.

**Fan toggle requires current temps:** When the user runs `ecobeectl fan on`, we need to include `coolHoldTemp` and `heatHoldTemp` in the setHold call. The client should first read the current desired temps from `GET /1/thermostat` runtime, then include them in the hold.

**Hold type:** Default to `"indefinite"`. Consider adding a `--hold-type` flag supporting `"indefinite"`, `"dateTime"`, `"nextTransition"`, `"holdHours"` in the future.

**Error handling:** ecobee returns `{"status": {"code": N, "message": "..."}}`. Code `0` is success. Any non-zero code should be surfaced as an error with the message.

**Rate limiting:** The ecobee API has rate limits. Implement retry with backoff on 429 responses (same pattern as eightctl's 429 handling with 2-second sleep).
