package client

import (
	"context"
	"encoding/json"
	"net/url"
)

type energyResponse struct {
	apiEnvelope
	Data json.RawMessage `json:"data"`
}

func (c *Client) GetEnergyReport(ctx context.Context, startDate, endDate string) (json.RawMessage, error) {
	if err := c.EnsureThermostatID(ctx); err != nil {
		return nil, err
	}
	query := url.Values{}
	payload := map[string]any{
		"selection": map[string]any{
			"selectionType":  "thermostats",
			"selectionMatch": c.ThermostatID,
			"includeAlerts":  true,
		},
		"startDate": startDate,
		"endDate":   endDate,
	}
	if err := addJSONQuery(query, "body", payload); err != nil {
		return nil, err
	}
	var resp energyResponse
	if err := c.do(ctx, "GET", "/1/energyIqReport", query, nil, &resp); err != nil {
		return nil, err
	}
	if err := checkStatus(resp.Status); err != nil {
		return nil, err
	}
	return resp.Data, nil
}
