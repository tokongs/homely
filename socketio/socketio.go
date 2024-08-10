// This package supports a tiny subset of the [Socket.IO] protocol.
// For now it only supports websockets, not long-polling.
//
// [Socket.IO]: https://socket.io/docs/v4/socket-io-protocol
package socketio

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"

	"github.com/coder/websocket"
	"golang.org/x/oauth2"
)

const (
	EngineIOVersion = "4"
	Transport       = "websocket"
)

// PacketType is the SocketIO packet type.
type PacketType int

const (
	PacketTypeConnect      PacketType = iota
	PacketTypeDisconnect              = iota
	PacketTypeEvent                   = iota
	PacketTypeAck                     = iota
	PacketTypeConnectError            = iota
	PacketTypeBinaryEvent             = iota
	PacketTypeBinaryAck               = iota
)

const (
	EIOPacketTypeOpen PacketType = iota
	EIOPacketTypeClose
	EIOPacketTypePing
	EIOPacketTypePong
	EIOPacketTypeMessage
	EIOPacketTypeUpgrade
	EIOPacketTypeNoop
)

// New creates a Client for receiving events from a Socket.IO server.
func New(server string, ts oauth2.TokenSource) *Client {
	return &Client{
		server:      server,
		namespace:   "/",
		tokenSource: ts,
	}
}

type Client struct {
	server      string
	namespace   string
	tokenSource oauth2.TokenSource
}

func (c *Client) HandleEvents(ctx context.Context, h func(name string, msg string) error) error {
	u, err := url.Parse(c.server)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}

	q := u.Query()
	q.Set("EIO", EngineIOVersion)
	q.Set("transport", Transport)

	if c.tokenSource != nil {
		t, err := c.tokenSource.Token()
		if err != nil {
			return fmt.Errorf("get token: %w", err)
		}

		q.Set("token", fmt.Sprintf("Bearer %s", t.AccessToken))
	}

	u.RawQuery = q.Encode()

	conn, _, err := websocket.Dial(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	defer func() {
		if err := conn.CloseNow(); err != nil {
			slog.Error("Errored while closing websocket connection", "error", err)
		}
	}()

	// SocketIO connection request
	if err := conn.Write(ctx, websocket.MessageText, []byte("40")); err != nil {
		return fmt.Errorf("socketio connect to namespace: %w", err)
	}

	for {
		_, b, err := conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		s := string(b)

		slog.Debug("Got websocket packet", "packet", s)

		// We only care about EngineIO packets. They start with the message type number
		if len(s) < 1 {
			slog.Debug("Packet has no data")
			continue
		}

		eioType, err := strconv.Atoi(string(s[0]))
		if err != nil {
			slog.Debug("Invalid EngineIO type", "type", s[0])
			continue
		}

		if PacketType(eioType) == EIOPacketTypePing {
			slog.Debug("Got EngineIO Ping, will Pong")
			if err := conn.Write(ctx, websocket.MessageText, []byte("3")); err != nil {
				return fmt.Errorf("eio pong: %w", err)
			}

			slog.Debug("Ponged")
			continue
		}

		if len(s) < 2 {
			// it has no data so we don't care
			slog.Debug("Message is not SocketIO message")
			continue
		}

		sioType, err := strconv.Atoi(string(s[1]))
		if err != nil {
			slog.Debug("Invalid SocketIO type", "type", s[1])
			continue
		}

		if PacketType(sioType) != PacketTypeEvent || len(s) < 3 {
			slog.Debug("Skipping non event SocketIO packet")
			continue
		}

		var values []json.RawMessage
		if err := json.Unmarshal([]byte(s[2:]), &values); err != nil {
			slog.Error("Could not unmarshal SocketIO event", "error", err)
			continue
		}

		if len(values) < 2 {
			slog.Error("Got unexpected number of values from SocketIO event", "values", values)
			continue
		}

		var name string
		if err := json.Unmarshal(values[0], &name); err != nil {
			slog.Error("Failed to unmarshal event name", "error", err)
			continue
		}

		data, err := values[1].MarshalJSON()
		if err != nil {
			slog.Error("Failed to handle message body", "error", err)
			continue
		}

		if err := h(name, string(data)); err != nil {
			return err
		}
	}
}
