package client

import (
	"context"
	"net/url"
)

type ThermostatSummary struct {
	ThermostatCount int      `json:"thermostatCount"`
	RevisionList    []string `json:"revisionList"`
	StatusList      []string `json:"statusList"`
}

type summaryResponse struct {
	apiEnvelope
	ThermostatSummary
}

func (c *Client) GetThermostatSummary(ctx context.Context) (*ThermostatSummary, error) {
	query := url.Values{}
	payload := map[string]any{
		"selection": map[string]any{
			"selectionType":          "thermostats",
			"selectionMatch":         "",
			"includeEquipmentStatus": true,
		},
	}
	if err := addJSONQuery(query, "json", payload); err != nil {
		return nil, err
	}
	var resp summaryResponse
	if err := c.do(ctx, "GET", "/1/thermostatSummary", query, nil, &resp); err != nil {
		return nil, err
	}
	if err := checkStatus(resp.Status); err != nil {
		return nil, err
	}
	return &resp.ThermostatSummary, nil
}
