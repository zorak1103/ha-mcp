// Package homeassistant provides WebSocket message types for Home Assistant API.
package homeassistant

import "encoding/json"

// WSMessage represents a base WebSocket message with ID and Type.
type WSMessage struct {
	ID   int64  `json:"id,omitempty"`
	Type string `json:"type"`
}

// WSAuthMessage is sent to authenticate with Home Assistant.
type WSAuthMessage struct {
	Type        string `json:"type"`
	AccessToken string `json:"access_token"`
}

// WSAuthRequired is received when connection requires authentication.
type WSAuthRequired struct {
	Type      string `json:"type"`
	HAVersion string `json:"ha_version"`
}

// WSAuthOK is received when authentication succeeds.
type WSAuthOK struct {
	Type      string `json:"type"`
	HAVersion string `json:"ha_version"`
}

// WSAuthInvalid is received when authentication fails.
type WSAuthInvalid struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// WSResultMessage represents a command result from Home Assistant.
type WSResultMessage struct {
	ID      int64           `json:"id"`
	Type    string          `json:"type"`
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *WSError        `json:"error,omitempty"`
}

// WSError represents an error in a WebSocket response.
type WSError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WSEventMessage represents an event message from Home Assistant.
type WSEventMessage struct {
	ID    int64   `json:"id"`
	Type  string  `json:"type"`
	Event WSEvent `json:"event"`
}

// WSEvent contains event data.
type WSEvent struct {
	EventType string         `json:"event_type"`
	Data      map[string]any `json:"data"`
	Origin    string         `json:"origin"`
	TimeFired string         `json:"time_fired"`
	Context   Context        `json:"context"`
}

// WSCommand represents a command to send to Home Assistant.
type WSCommand struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
	// Additional fields are added dynamically based on command type
}

// WSCommandWithPayload represents a command with additional payload data.
type WSCommandWithPayload struct {
	ID      int64          `json:"id"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"-"`
}

// MarshalJSON implements custom JSON marshaling to flatten payload into the message.
func (c *WSCommandWithPayload) MarshalJSON() ([]byte, error) {
	// Create a map with base fields
	m := map[string]any{
		"id":   c.ID,
		"type": c.Type,
	}
	// Merge payload fields
	for k, v := range c.Payload {
		m[k] = v
	}
	return json.Marshal(m)
}

// WSCallServiceCommand represents a call_service command.
type WSCallServiceCommand struct {
	ID          int64          `json:"id"`
	Type        string         `json:"type"`
	Domain      string         `json:"domain"`
	Service     string         `json:"service"`
	ServiceData map[string]any `json:"service_data,omitempty"`
	Target      *WSTarget      `json:"target,omitempty"`
}

// WSTarget specifies targets for service calls.
type WSTarget struct {
	EntityID []string `json:"entity_id,omitempty"`
	DeviceID []string `json:"device_id,omitempty"`
	AreaID   []string `json:"area_id,omitempty"`
}

// WSSubscribeEventsCommand subscribes to events.
type WSSubscribeEventsCommand struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	EventType string `json:"event_type,omitempty"`
}

// ParseMessageType extracts the message type from a raw JSON message.
func ParseMessageType(data []byte) (string, error) {
	var msg struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return "", err
	}
	return msg.Type, nil
}

// ParseMessageID extracts the message ID from a raw JSON message.
func ParseMessageID(data []byte) (int64, error) {
	var msg struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return 0, err
	}
	return msg.ID, nil
}
