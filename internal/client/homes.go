package client

import (
	"context"
	"encoding/json"
	"fmt"
)

const homesQuery = `query SPHomesQuery { homes { id name permissions members { id role } devices { thermostats { id } lightSwitches { id } } } unassigned { thermostats { id } lightSwitches { id } } }`

type HomesResponse struct {
	Data json.RawMessage `json:"data"`
}

type HomesData struct {
	Homes      []Home      `json:"homes"`
	Unassigned HomeDevices `json:"unassigned"`
}

type Home struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Permissions []string     `json:"permissions"`
	Members     []HomeMember `json:"members"`
	Devices     HomeDevices  `json:"devices"`
}

type HomeMember struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

type HomeDevices struct {
	Thermostats   []HomeDevice `json:"thermostats"`
	LightSwitches []HomeDevice `json:"lightSwitches"`
}

type HomeDevice struct {
	ID string `json:"id"`
}

func (c *Client) GetHomes(ctx context.Context) (json.RawMessage, error) {
	var resp HomesResponse
	if err := c.doGraphQL(ctx, homesQuery, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetHomesData(ctx context.Context) (*HomesData, error) {
	raw, err := c.GetHomes(ctx)
	if err != nil {
		return nil, err
	}
	var data HomesData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decode homes data: %w", err)
	}
	return &data, nil
}
