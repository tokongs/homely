package homely

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/tokongs/homely/socketio"
	"golang.org/x/oauth2"
)

const defaultBaseURL = "https://sdk.iotiliti.cloud"

type Client struct {
	http        http.Client
	tokenSource oauth2.TokenSource
	baseURL     string
}

type Config struct {
	Username string
	Password string
	BaseURL  string
}

type Location struct {
	Name          string    `json:"name"`
	LocationID    uuid.UUID `json:"locationId"`
	UserID        uuid.UUID `json:"userId"`
	GatewaySerial string    `json:"gatewayserial"`
	PartnerCode   int       `json:"partnerCode"`
}

type LocationDetails struct {
	LocationID         uuid.UUID `json:"locationID"`
	GatewaySerial      string    `json:"gatewayserial"`
	Name               string    `json:"name"`
	AlarmState         string    `json:"alarmState"`
	UserRoleAtLocation string    `json:"userRoleAtLocation"`
	Devices            []Device  `json:"devices"`
}

type Device struct {
	ID           uuid.UUID          `json:"id"`
	Name         string             `json:"name"`
	SerialNumber string             `json:"serialNumber"`
	Location     string             `json:"location"`
	Online       bool               `json:"online"`
	ModelID      uuid.UUID          `json:"modelId"`
	ModelName    string             `json:"modelName"`
	Features     map[string]Feature `json:"features"`
}

type Feature struct {
	States map[string]State `json:"states"`
}

type State struct {
	Value       any       `json:"value"`
	LastUpdated time.Time `json:"lastUpdated"`
}

type Event struct {
	Type string `json:"type"`
	Data EventData `json:"data"`
}

type EventData struct {
	DeviceID       uuid.UUID `json:"deviceId"`
	GatewayID      uuid.UUID `json:"gatewayId"`
	LocationID     uuid.UUID `json:"locationId"`
	ModelID        uuid.UUID `json:"modelId"`
	RootLocationID uuid.UUID `json:"rootLocationId"`
	Changes        []Change  `json:"changes"`
	PartnerCode    int       `json:"partnerCode"`
}

type Change struct {
	Feature     string    `json:"feature"`
	StateName   string    `json:"stateName"`
	Value       any       `json:"value"`
	LastUpdated time.Time `json:"lastUpdated"`
}

func New(c Config) *Client {
	if c.BaseURL == "" {
		c.BaseURL = defaultBaseURL
	}

	ts := oauth2.ReuseTokenSource(nil, &tokenSource{
		baseURL:  c.BaseURL,
		username: c.Username,
		password: c.Password,
	})

	return &Client{
		tokenSource: ts,
		http:        *oauth2.NewClient(context.Background(), ts),
		baseURL:     c.BaseURL,
	}
}

func (c *Client) Locations(ctx context.Context) ([]Location, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "homely/locations", nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	l := []Location{}
	if err := c.doAndDecode(req, &l); err != nil {
		return nil, err
	}

	return l, nil
}

func (c *Client) LocationDetails(ctx context.Context, locationID uuid.UUID) (LocationDetails, error) {
	var l LocationDetails

	req, err := c.newRequest(ctx, http.MethodGet, fmt.Sprintf("homely/home/%s", locationID), nil)
	if err != nil {
		return l, fmt.Errorf("new request: %w", err)
	}

	if err := c.doAndDecode(req, &l); err != nil {
		return l, err
	}

	return l, nil
}

func (c *Client) Stream(ctx context.Context, locationID uuid.UUID, h func(e Event)) error {
	sio := socketio.New(fmt.Sprintf("%s/socket.io/?locationId=%s", c.baseURL, locationID), c.tokenSource)
	return sio.HandleEvents(ctx, func(name, msg string) error {
		if name != "event" {
			slog.Warn("Got non event event", "name", name)
			return nil
		}

		var e Event
		if err := json.Unmarshal([]byte(msg), &e); err != nil {
			return fmt.Errorf("unmarshal event: %w", err)
		}

		h(e)

		return nil
	})
}

func (c *Client) newRequest(ctx context.Context, method string, resource string, body io.Reader) (*http.Request, error) {
	path, err := url.JoinPath(c.baseURL, resource)
	if err != nil {
		return nil, fmt.Errorf("join path: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, path, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application-json")

	return req, nil
}

func (c *Client) doAndDecode(req *http.Request, v any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("request failed and so did reading the body: %w", err)
		}

		return fmt.Errorf(string(msg))
	}

	return json.NewDecoder(resp.Body).Decode(v)
}
