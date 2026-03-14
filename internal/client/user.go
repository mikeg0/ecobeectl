package client

import (
	"context"
	"encoding/json"
	"net/url"
)

type userResponse struct {
	apiEnvelope
	User json.RawMessage `json:"user"`
}

func (c *Client) GetUser(ctx context.Context) (json.RawMessage, error) {
	query := url.Values{}
	if err := addJSONQuery(query, "json", map[string]any{"userName": c.Email}); err != nil {
		return nil, err
	}
	var resp userResponse
	if err := c.do(ctx, "GET", "/1/user", query, nil, &resp); err != nil {
		return nil, err
	}
	if err := checkStatus(resp.Status); err != nil {
		return nil, err
	}
	return resp.User, nil
}
