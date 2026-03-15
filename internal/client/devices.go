package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type Device struct {
	Raw map[string]any
}

func (d Device) Identifier() string {
	for _, key := range []string{"identifier", "deviceId", "id"} {
		if value, ok := d.Raw[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func (d Device) Name() string {
	for _, key := range []string{"name", "deviceName", "nickname"} {
		if value, ok := d.Raw[key].(string); ok && value != "" {
			return value
		}
	}
	return d.Identifier()
}

func (d Device) Type() string {
	for _, key := range []string{"type", "deviceType", "category"} {
		if value, ok := d.Raw[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func (d Device) IsThermostat() bool {
	joined := strings.ToLower(d.Type() + " " + d.Name())
	return strings.Contains(joined, "thermostat") || strings.HasPrefix(strings.ToLower(d.Identifier()), "5")
}

const devicesQuery = `query SPHomesQuery {
  homes {
    id
    name
    devices {
      thermostats { id name }
      lightSwitches { id name }
    }
  }
  unassigned {
    thermostats { id name }
    lightSwitches { id name }
  }
}`

type devicesGraphQLResponse struct {
	Data struct {
		Homes []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Devices struct {
				Thermostats   []struct{ ID, Name string } `json:"thermostats"`
				LightSwitches []struct{ ID, Name string } `json:"lightSwitches"`
			} `json:"devices"`
		} `json:"homes"`
		Unassigned struct {
			Thermostats   []struct{ ID, Name string } `json:"thermostats"`
			LightSwitches []struct{ ID, Name string } `json:"lightSwitches"`
		} `json:"unassigned"`
	} `json:"data"`
}

func (c *Client) ListDevices(ctx context.Context) ([]Device, error) {
	var resp devicesGraphQLResponse
	if err := c.doGraphQL(ctx, devicesQuery, &resp); err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	var devices []Device
	for _, home := range resp.Data.Homes {
		for _, t := range home.Devices.Thermostats {
			devices = append(devices, Device{Raw: map[string]any{
				"id": t.ID, "name": t.Name, "type": "thermostat", "home": home.Name,
			}})
		}
		for _, ls := range home.Devices.LightSwitches {
			devices = append(devices, Device{Raw: map[string]any{
				"id": ls.ID, "name": ls.Name, "type": "lightSwitch", "home": home.Name,
			}})
		}
	}
	addUnassigned := func(items []struct{ ID, Name string }, devType string) {
		for _, item := range items {
			devices = append(devices, Device{Raw: map[string]any{
				"id": item.ID, "name": item.Name, "type": devType, "home": "unassigned",
			}})
		}
	}
	addUnassigned(resp.Data.Unassigned.Thermostats, "thermostat")
	addUnassigned(resp.Data.Unassigned.LightSwitches, "lightSwitch")
	return devices, nil
}

// ListDevicesJSON returns the raw JSON from the GraphQL homes query for debugging.
func (c *Client) ListDevicesJSON(ctx context.Context) (json.RawMessage, error) {
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.doGraphQL(ctx, devicesQuery, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}
