package client

import (
	"context"
	"net/url"
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

type devicesResponse struct {
	Count   int              `json:"count"`
	Devices []map[string]any `json:"devices"`
}

func (c *Client) ListDevices(ctx context.Context) ([]Device, error) {
	query := url.Values{"format": []string{"json"}}
	var resp devicesResponse
	if err := c.do(ctx, "GET", "/ea/devices/ls", query, nil, &resp); err != nil {
		return nil, err
	}
	devices := make([]Device, 0, len(resp.Devices))
	for _, raw := range resp.Devices {
		devices = append(devices, Device{Raw: raw})
	}
	return devices, nil
}
