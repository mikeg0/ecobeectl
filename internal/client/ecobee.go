package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"github.com/mikeg/ecobeectl/internal/tokencache"
)

const (
	DefaultAPIBaseURL = "https://api.ecobee.com"
	DefaultAuthURL    = "https://auth.ecobee.com/oauth/token"
	DefaultGraphQLURL = "https://beehive.ecobee.com/graphql"
	DefaultClientID   = "183eORFPlXyz9BbDZwqexHPBQoVjgadh"
	DefaultAudience   = "https://prod.ecobee.com/api/v1"
	DefaultScopes     = "openid smartWrite piiWrite piiRead smartRead deleteGrants offline_access"
)

var sleep = time.Sleep

type Client struct {
	Email          string
	Password       string
	ClientID       string
	ClientIDSource string
	ThermostatID   string
	HTTP           *http.Client
	APIBaseURL     string
	AuthURL        string
	GraphQLURL     string
	Verbose        bool
	Cache          tokencache.Store

	token        string
	refreshToken string
	tokenExp     time.Time
	accountID    string
}

type APIStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type apiEnvelope struct {
	Status APIStatus `json:"status"`
}

type authResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type AuthError struct {
	Message             string
	UpstreamError       string
	UpstreamDescription string
}

func (e *AuthError) Error() string {
	return e.Message
}

func New(email, password, thermostatID, clientID string, cache tokencache.Store) *Client {
	if clientID == "" {
		clientID = DefaultClientID
	}
	if cache == nil {
		panic("client cache store must not be nil")
	}
	return &Client{
		Email:        email,
		Password:     password,
		ClientID:     clientID,
		ThermostatID: thermostatID,
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
		},
		APIBaseURL: DefaultAPIBaseURL,
		AuthURL:    DefaultAuthURL,
		GraphQLURL: DefaultGraphQLURL,
		Cache:      cache,
	}
}

func (c *Client) Authenticate(ctx context.Context) error {
	if strings.TrimSpace(c.Email) == "" {
		return fmt.Errorf("email is required for authentication")
	}
	if c.Password == "" {
		return fmt.Errorf("password is required because no cached token is available")
	}

	body := map[string]any{
		"grant_type": "password",
		"client_id":  c.ClientID,
		"username":   c.Email,
		"password":   c.Password,
		"audience":   DefaultAudience,
		"scope":      DefaultScopes,
	}
	var resp authResponse
	if err := c.postAuth(ctx, body, &resp); err != nil {
		return err
	}
	return c.applyAuthResponse(resp)
}

func (c *Client) refreshAuth(ctx context.Context) error {
	if c.refreshToken == "" {
		return fmt.Errorf("refresh token is not available")
	}
	body := map[string]any{
		"grant_type":    "refresh_token",
		"client_id":     c.ClientID,
		"refresh_token": c.refreshToken,
	}
	var resp authResponse
	if err := c.postAuth(ctx, body, &resp); err != nil {
		return err
	}
	if resp.RefreshToken == "" {
		resp.RefreshToken = c.refreshToken
	}
	return c.applyAuthResponse(resp)
}

func (c *Client) applyAuthResponse(resp authResponse) error {
	if resp.AccessToken == "" {
		return fmt.Errorf("auth response did not include an access token")
	}
	c.token = resp.AccessToken
	c.refreshToken = resp.RefreshToken
	c.tokenExp = time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second).Add(-1 * time.Minute)
	if accountID, err := accountIDFromJWT(resp.AccessToken); err == nil {
		c.accountID = accountID
	}
	return c.Cache.Save(c.identity(), tokencache.CachedToken{
		AccessToken:  c.token,
		RefreshToken: c.refreshToken,
		ExpiresAt:    c.tokenExp,
		AccountID:    c.accountID,
	})
}

func (c *Client) ensureToken(ctx context.Context) error {
	if c.token != "" && time.Now().Before(c.tokenExp) {
		return nil
	}
	cached, err := c.Cache.Load(c.identity())
	if err == nil {
		c.token = cached.AccessToken
		c.refreshToken = cached.RefreshToken
		c.tokenExp = cached.ExpiresAt
		c.accountID = cached.AccountID
		if c.token != "" && time.Now().Before(c.tokenExp) {
			return nil
		}
		if c.refreshToken != "" {
			if err := c.refreshAuth(ctx); err == nil {
				return nil
			}
		}
	}
	return c.Authenticate(ctx)
}

func (c *Client) ClearCachedToken() error {
	c.token = ""
	c.refreshToken = ""
	c.tokenExp = time.Time{}
	return c.Cache.Clear(c.identity())
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	var requestBody []byte

	for attempt := 0; attempt < 3; attempt++ {
		if err := c.ensureToken(ctx); err != nil {
			return err
		}
		var bodyReader io.Reader
		if body != nil {
			if requestBody == nil {
				data, err := json.Marshal(body)
				if err != nil {
					return err
				}
				requestBody = data
			}
			bodyReader = bytes.NewReader(requestBody)
		}
		req, err := http.NewRequestWithContext(ctx, method, joinURL(c.APIBaseURL, path, query), bodyReader)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		if c.Verbose {
			log.Debugf("request %s %s", method, req.URL.String())
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}

		if resp.StatusCode == http.StatusUnauthorized {
			_ = c.ClearCachedToken()
			if attempt == 0 {
				continue
			}
			return fmt.Errorf("authentication failed after retry")
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		if resp.StatusCode >= 400 {
			return fmt.Errorf("ecobee API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
		}
		if out == nil {
			return nil
		}
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		return nil
	}
	return fmt.Errorf("request failed after retries")
}

func (c *Client) doGraphQL(ctx context.Context, query string, out any) error {
	if err := c.ensureToken(ctx); err != nil {
		return err
	}
	body := map[string]string{"query": query}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.GraphQLURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("graphql returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) EnsureThermostatID(ctx context.Context) error {
	if c.ThermostatID != "" {
		return nil
	}
	ids, err := c.discoverThermostatIDs(ctx)
	if err != nil {
		return err
	}
	switch len(ids) {
	case 0:
		return fmt.Errorf("no thermostats were found for this account")
	case 1:
		c.ThermostatID = ids[0]
		return nil
	default:
		return fmt.Errorf("multiple thermostats found; set thermostat_id in config or pass --thermostat-id: %s", strings.Join(ids, ", "))
	}
}

func (c *Client) discoverThermostatIDs(ctx context.Context) ([]string, error) {
	ids, err := c.discoverThermostatIDsFromHomes(ctx)
	if err == nil && len(ids) > 0 {
		return ids, nil
	}
	devices, err := c.ListDevices(ctx)
	if err != nil {
		if len(ids) > 0 {
			return ids, nil
		}
		return nil, err
	}
	var thermostats []Device
	for _, device := range devices {
		if device.IsThermostat() {
			thermostats = append(thermostats, device)
		}
	}
	resolved := make([]string, 0, len(thermostats))
	for _, device := range thermostats {
		id := device.Identifier()
		if id == "" {
			continue
		}
		resolved = append(resolved, id)
	}
	return uniqueStrings(resolved), nil
}

func (c *Client) discoverThermostatIDsFromHomes(ctx context.Context) ([]string, error) {
	data, err := c.GetHomesData(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0)
	for _, home := range data.Homes {
		for _, thermostat := range home.Devices.Thermostats {
			if thermostat.ID != "" {
				ids = append(ids, thermostat.ID)
			}
		}
	}
	for _, thermostat := range data.Unassigned.Thermostats {
		if thermostat.ID != "" {
			ids = append(ids, thermostat.ID)
		}
	}
	return uniqueStrings(ids), nil
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (c *Client) identity() tokencache.Identity {
	return tokencache.Identity{
		AuthURL:  c.AuthURL,
		ClientID: c.ClientID,
		Email:    c.Email,
	}
}

func (c *Client) postAuth(ctx context.Context, body map[string]any, out *authResponse) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.AuthURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return err
	}
	if resp.StatusCode >= 400 || out.Error != "" {
		return c.authError(*out, resp.StatusCode)
	}
	return nil
}

func (c *Client) authError(resp authResponse, statusCode int) error {
	lower := strings.ToLower(resp.Error + " " + resp.ErrorDescription)
	msg := "authentication failed"
	if strings.Contains(lower, "unauthorized_client") || strings.Contains(lower, "invalid_client") {
		msg = "authentication failed because the ecobee portal client ID may have changed; set client_id in ~/.config/ecobeectl/config.yaml, set ECOBEECTL_CLIENT_ID, pass --client-id, or run `ecobeectl check-client-id`"
	}
	if statusCode > 0 && msg == "authentication failed" {
		msg = fmt.Sprintf("authentication failed with HTTP %d", statusCode)
	}
	if c.Verbose && (resp.Error != "" || resp.ErrorDescription != "") {
		msg = fmt.Sprintf("%s: %s (%s)", msg, resp.Error, resp.ErrorDescription)
	}
	return &AuthError{
		Message:             msg,
		UpstreamError:       resp.Error,
		UpstreamDescription: resp.ErrorDescription,
	}
}

func checkStatus(status APIStatus) error {
	if status.Code != 0 {
		if status.Message == "" {
			return fmt.Errorf("ecobee API returned status code %d", status.Code)
		}
		return fmt.Errorf("ecobee API returned status code %d: %s", status.Code, status.Message)
	}
	return nil
}

func addJSONQuery(query url.Values, key string, payload any) error {
	if query == nil {
		return fmt.Errorf("query values must not be nil")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	query.Set("format", "json")
	query.Set(key, string(data))
	query.Set("_timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))
	return nil
}

func joinURL(baseURL, path string, query url.Values) string {
	u, _ := url.Parse(baseURL)
	u.Path = strings.TrimRight(u.Path, "/") + path
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	return u.String()
}

func accountIDFromJWT(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", errors.New("invalid JWT")
	}
	payload, err := decodeJWTPart(parts[1])
	if err != nil {
		return "", err
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", err
	}
	accountID, _ := claims["https://claims.ecobee.com/ecobee_account_id"].(string)
	return accountID, nil
}

func decodeJWTPart(part string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(part)
}

type ClientIDDetector struct {
	HTTP              *http.Client
	HomepageURL       string
	ConsumerPortalURL string
}

func (d ClientIDDetector) Detect(ctx context.Context) (string, string, error) {
	httpClient := d.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	homepageURL := d.HomepageURL
	if homepageURL == "" {
		homepageURL = "https://www.ecobee.com/en-us/"
	}
	consumerURL := d.ConsumerPortalURL
	if consumerURL == "" {
		consumerURL = "https://www.ecobee.com/consumerportal/index.html"
	}

	if value, err := detectClientIDFromURL(ctx, httpClient, homepageURL); err == nil && value != "" {
		return value, "homepage", nil
	}
	return detectClientIDFromConsumerPortal(ctx, httpClient, consumerURL)
}

func detectClientIDFromURL(ctx context.Context, httpClient *http.Client, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	match := regexp.MustCompile(`client_id=([A-Za-z0-9]+)`).FindSubmatch(body)
	if len(match) == 2 {
		return string(match[1]), nil
	}
	return "", fmt.Errorf("client_id not found")
}

func detectClientIDFromConsumerPortal(ctx context.Context, httpClient *http.Client, indexURL string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexURL, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	indexBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	scriptMatch := regexp.MustCompile(`(scripts\.[a-zA-Z0-9]+\.js)`).FindSubmatch(indexBody)
	if len(scriptMatch) != 2 {
		return "", "", fmt.Errorf("consumer portal bundle not found")
	}
	scriptURL, err := url.Parse(indexURL)
	if err != nil {
		return "", "", err
	}
	scriptURL.Path = path.Join(path.Dir(scriptURL.Path), string(scriptMatch[1]))
	jsReq, err := http.NewRequestWithContext(ctx, http.MethodGet, scriptURL.String(), nil)
	if err != nil {
		return "", "", err
	}
	jsResp, err := httpClient.Do(jsReq)
	if err != nil {
		return "", "", err
	}
	defer jsResp.Body.Close()
	jsBody, err := io.ReadAll(jsResp.Body)
	if err != nil {
		return "", "", err
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`client_id["':= ]+["']([A-Za-z0-9]+)["']`),
		regexp.MustCompile(`authReq["':= ]+[^}]*client_id["':= ]+["']([A-Za-z0-9]+)["']`),
	}
	for _, pattern := range patterns {
		match := pattern.FindSubmatch(jsBody)
		if len(match) == 2 {
			return string(match[1]), "consumer-portal-js", nil
		}
	}
	return "", "", fmt.Errorf("client_id not found in consumer portal bundle")
}
