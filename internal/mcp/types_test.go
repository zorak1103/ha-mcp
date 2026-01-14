// Package mcp implements the Model Context Protocol (MCP) server.
package mcp

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNewTextContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want ContentBlock
	}{
		{
			name: "simple text",
			text: "Hello, world!",
			want: ContentBlock{
				Type: "text",
				Text: "Hello, world!",
			},
		},
		{
			name: "empty text",
			text: "",
			want: ContentBlock{
				Type: "text",
				Text: "",
			},
		},
		{
			name: "text with special characters",
			text: "Line1\nLine2\tTab",
			want: ContentBlock{
				Type: "text",
				Text: "Line1\nLine2\tTab",
			},
		},
		{
			name: "unicode text",
			text: "Hello 世界 🌍",
			want: ContentBlock{
				Type: "text",
				Text: "Hello 世界 🌍",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NewTextContent(tt.text)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("NewTextContent() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNewErrorResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      json.RawMessage
		code    ErrorCode
		message string
		data    any
	}{
		{
			name:    "with numeric id",
			id:      json.RawMessage(`1`),
			code:    ParseError,
			message: "parse error",
			data:    nil,
		},
		{
			name:    "with string id",
			id:      json.RawMessage(`"abc-123"`),
			code:    InvalidRequest,
			message: "invalid request",
			data:    "additional info",
		},
		{
			name:    "with nil id",
			id:      nil,
			code:    MethodNotFound,
			message: "method not found",
			data:    nil,
		},
		{
			name:    "with complex data",
			id:      json.RawMessage(`42`),
			code:    InternalError,
			message: "internal error",
			data:    map[string]string{"field": "value"},
		},
		{
			name:    "mcp specific error - tool not found",
			id:      json.RawMessage(`1`),
			code:    ToolNotFound,
			message: "tool not found: my_tool",
			data:    nil,
		},
		{
			name:    "mcp specific error - resource not found",
			id:      json.RawMessage(`1`),
			code:    ResourceNotFound,
			message: "resource not found: test://uri",
			data:    nil,
		},
		{
			name:    "mcp specific error - tool execution error",
			id:      json.RawMessage(`1`),
			code:    ToolExecutionErr,
			message: "tool execution failed",
			data:    "stack trace here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NewErrorResponse(tt.id, tt.code, tt.message, tt.data)

			if got.JSONRPC != JSONRPCVersion {
				t.Errorf("JSONRPC = %q, want %q", got.JSONRPC, JSONRPCVersion)
			}
			if string(got.ID) != string(tt.id) {
				t.Errorf("ID = %s, want %s", got.ID, tt.id)
			}
			if got.Result != nil {
				t.Error("Result should be nil for error response")
			}
			if got.Error == nil {
				t.Fatal("Error should not be nil")
			}
			if got.Error.Code != tt.code {
				t.Errorf("Error.Code = %d, want %d", got.Error.Code, tt.code)
			}
			if got.Error.Message != tt.message {
				t.Errorf("Error.Message = %q, want %q", got.Error.Message, tt.message)
			}
			if diff := cmp.Diff(tt.data, got.Error.Data); diff != "" {
				t.Errorf("Error.Data mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNewSuccessResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		id     json.RawMessage
		result any
	}{
		{
			name:   "with numeric id and string result",
			id:     json.RawMessage(`1`),
			result: "success",
		},
		{
			name:   "with string id and struct result",
			id:     json.RawMessage(`"request-123"`),
			result: InitializeResult{ProtocolVersion: "2024-11-05"},
		},
		{
			name:   "with nil result",
			id:     json.RawMessage(`1`),
			result: nil,
		},
		{
			name:   "with empty struct result",
			id:     json.RawMessage(`1`),
			result: PingResult{},
		},
		{
			name:   "with tools list result",
			id:     json.RawMessage(`5`),
			result: ToolsListResult{Tools: []Tool{{Name: "test"}}},
		},
		{
			name: "with resources list result",
			id:   json.RawMessage(`6`),
			result: ResourcesListResult{Resources: []Resource{
				{URI: "test://a", Name: "Test A"},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NewSuccessResponse(tt.id, tt.result)

			if got.JSONRPC != JSONRPCVersion {
				t.Errorf("JSONRPC = %q, want %q", got.JSONRPC, JSONRPCVersion)
			}
			if string(got.ID) != string(tt.id) {
				t.Errorf("ID = %s, want %s", got.ID, tt.id)
			}
			if got.Error != nil {
				t.Errorf("Error should be nil, got %+v", got.Error)
			}
			if diff := cmp.Diff(tt.result, got.Result); diff != "" {
				t.Errorf("Result mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestErrorCodeValues(t *testing.T) {
	t.Parallel()

	// Standard JSON-RPC 2.0 error codes
	if ParseError != -32700 {
		t.Errorf("ParseError = %d, want -32700", ParseError)
	}
	if InvalidRequest != -32600 {
		t.Errorf("InvalidRequest = %d, want -32600", InvalidRequest)
	}
	if MethodNotFound != -32601 {
		t.Errorf("MethodNotFound = %d, want -32601", MethodNotFound)
	}
	if InvalidParams != -32602 {
		t.Errorf("InvalidParams = %d, want -32602", InvalidParams)
	}
	if InternalError != -32603 {
		t.Errorf("InternalError = %d, want -32603", InternalError)
	}

	// MCP-specific error codes
	if ResourceNotFound != -32001 {
		t.Errorf("ResourceNotFound = %d, want -32001", ResourceNotFound)
	}
	if ToolNotFound != -32002 {
		t.Errorf("ToolNotFound = %d, want -32002", ToolNotFound)
	}
	if ToolExecutionErr != -32003 {
		t.Errorf("ToolExecutionErr = %d, want -32003", ToolExecutionErr)
	}
}

func TestMethodConstants(t *testing.T) {
	t.Parallel()

	expectedMethods := map[string]string{
		"MethodInitialize":    "initialize",
		"MethodInitialized":   "notifications/initialized",
		"MethodToolsList":     "tools/list",
		"MethodToolsCall":     "tools/call",
		"MethodResourcesList": "resources/list",
		"MethodResourcesRead": "resources/read",
		"MethodPromptsList":   "prompts/list",
		"MethodPromptsGet":    "prompts/get",
		"MethodLoggingSetLvl": "logging/setLevel",
		"MethodPing":          "ping",
		"MethodCanceled":      "notifications/cancelled", //nolint:misspell // MCP protocol-defined value
		"MethodProgress":      "notifications/progress",
	}

	actuals := map[string]string{
		"MethodInitialize":    MethodInitialize,
		"MethodInitialized":   MethodInitialized,
		"MethodToolsList":     MethodToolsList,
		"MethodToolsCall":     MethodToolsCall,
		"MethodResourcesList": MethodResourcesList,
		"MethodResourcesRead": MethodResourcesRead,
		"MethodPromptsList":   MethodPromptsList,
		"MethodPromptsGet":    MethodPromptsGet,
		"MethodLoggingSetLvl": MethodLoggingSetLvl,
		"MethodPing":          MethodPing,
		"MethodCanceled":      MethodCanceled,
		"MethodProgress":      MethodProgress,
	}

	for name, expected := range expectedMethods {
		actual, ok := actuals[name]
		if !ok {
			t.Errorf("Missing constant %s", name)
			continue
		}
		if actual != expected {
			t.Errorf("%s = %q, want %q", name, actual, expected)
		}
	}
}

func TestRequest_JSONSerialization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		request  Request
		wantJSON string
	}{
		{
			name: "request with params",
			request: Request{
				JSONRPC: JSONRPCVersion,
				ID:      json.RawMessage(`1`),
				Method:  MethodToolsList,
				Params:  json.RawMessage(`{"cursor": null}`),
			},
			wantJSON: `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"cursor": null}}`,
		},
		{
			name: "notification without id",
			request: Request{
				JSONRPC: JSONRPCVersion,
				Method:  MethodInitialized,
			},
			wantJSON: `{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(tt.request)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			// Verify it can be unmarshaled back
			var decoded Request
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			if decoded.JSONRPC != tt.request.JSONRPC {
				t.Errorf("JSONRPC = %q, want %q", decoded.JSONRPC, tt.request.JSONRPC)
			}
			if decoded.Method != tt.request.Method {
				t.Errorf("Method = %q, want %q", decoded.Method, tt.request.Method)
			}
		})
	}
}

func TestResponse_JSONSerialization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response Response
	}{
		{
			name: "success response",
			response: Response{
				JSONRPC: JSONRPCVersion,
				ID:      json.RawMessage(`1`),
				Result:  map[string]string{"status": "ok"},
			},
		},
		{
			name: "error response",
			response: Response{
				JSONRPC: JSONRPCVersion,
				ID:      json.RawMessage(`2`),
				Error: &ErrorObject{
					Code:    InvalidParams,
					Message: "invalid parameters",
					Data:    "field 'x' is required",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(tt.response)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			var decoded Response
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			if decoded.JSONRPC != tt.response.JSONRPC {
				t.Errorf("JSONRPC = %q, want %q", decoded.JSONRPC, tt.response.JSONRPC)
			}
			if string(decoded.ID) != string(tt.response.ID) {
				t.Errorf("ID = %s, want %s", decoded.ID, tt.response.ID)
			}
		})
	}
}

func TestTool_JSONSerialization(t *testing.T) {
	t.Parallel()

	tool := Tool{
		Name:        "get_state",
		Description: "Get the state of an entity",
		InputSchema: JSONSchema{
			Type:        "object",
			Description: "Parameters for getting entity state",
			Properties: map[string]JSONSchema{
				"entity_id": {
					Type:        "string",
					Description: "The entity ID",
				},
			},
			Required: []string{"entity_id"},
		},
	}

	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded Tool
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Name != tool.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, tool.Name)
	}
	if decoded.Description != tool.Description {
		t.Errorf("Description = %q, want %q", decoded.Description, tool.Description)
	}
	if decoded.InputSchema.Type != tool.InputSchema.Type {
		t.Errorf("InputSchema.Type = %q, want %q", decoded.InputSchema.Type, tool.InputSchema.Type)
	}
	if len(decoded.InputSchema.Required) != len(tool.InputSchema.Required) {
		t.Errorf("InputSchema.Required length = %d, want %d", len(decoded.InputSchema.Required), len(tool.InputSchema.Required))
	}
}

func TestResource_JSONSerialization(t *testing.T) {
	t.Parallel()

	resource := Resource{
		URI:         "ha://entities/light.living_room",
		Name:        "Living Room Light",
		Description: "State of the living room light",
		MimeType:    "application/json",
	}

	data, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded Resource
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if diff := cmp.Diff(resource, decoded); diff != "" {
		t.Errorf("Resource mismatch (-want +got):\n%s", diff)
	}
}

func TestContentBlock_JSONSerialization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content ContentBlock
	}{
		{
			name: "text content",
			content: ContentBlock{
				Type: "text",
				Text: "Hello, world!",
			},
		},
		{
			name: "image content",
			content: ContentBlock{
				Type:     "image",
				MimeType: "image/png",
				Data:     "base64encodeddata",
			},
		},
		{
			name: "resource content",
			content: ContentBlock{
				Type: "resource",
				URI:  "ha://entities/sensor.temperature",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(tt.content)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			var decoded ContentBlock
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			if diff := cmp.Diff(tt.content, decoded); diff != "" {
				t.Errorf("ContentBlock mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestInitializeParams_JSONSerialization(t *testing.T) {
	t.Parallel()

	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities: ClientCapabilities{
			Roots: &RootsCapability{ListChanged: true},
		},
		ClientInfo: Implementation{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded InitializeParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.ProtocolVersion != params.ProtocolVersion {
		t.Errorf("ProtocolVersion = %q, want %q", decoded.ProtocolVersion, params.ProtocolVersion)
	}
	if decoded.ClientInfo.Name != params.ClientInfo.Name {
		t.Errorf("ClientInfo.Name = %q, want %q", decoded.ClientInfo.Name, params.ClientInfo.Name)
	}
}

func TestInitializeResult_JSONSerialization(t *testing.T) {
	t.Parallel()

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{ListChanged: false},
			Resources: &ResourcesCapability{
				Subscribe:   true,
				ListChanged: false,
			},
		},
		ServerInfo: Implementation{
			Name:    "test-server",
			Version: "2.0.0",
		},
		Instructions: "Test instructions",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded InitializeResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.ProtocolVersion != result.ProtocolVersion {
		t.Errorf("ProtocolVersion = %q, want %q", decoded.ProtocolVersion, result.ProtocolVersion)
	}
	if decoded.ServerInfo.Name != result.ServerInfo.Name {
		t.Errorf("ServerInfo.Name = %q, want %q", decoded.ServerInfo.Name, result.ServerInfo.Name)
	}
	if decoded.Instructions != result.Instructions {
		t.Errorf("Instructions = %q, want %q", decoded.Instructions, result.Instructions)
	}
}

func TestToolsCallResult_JSONSerialization(t *testing.T) {
	t.Parallel()

	result := ToolsCallResult{
		Content: []ContentBlock{
			NewTextContent("Result 1"),
			NewTextContent("Result 2"),
		},
		IsError: false,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded ToolsCallResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(decoded.Content) != len(result.Content) {
		t.Errorf("Content length = %d, want %d", len(decoded.Content), len(result.Content))
	}
	if decoded.IsError != result.IsError {
		t.Errorf("IsError = %v, want %v", decoded.IsError, result.IsError)
	}
}

func TestResourcesReadResult_JSONSerialization(t *testing.T) {
	t.Parallel()

	result := ResourcesReadResult{
		Contents: []ResourceContent{
			{
				URI:      "test://resource",
				MimeType: "application/json",
				Text:     `{"key": "value"}`,
			},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded ResourcesReadResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(decoded.Contents) != 1 {
		t.Fatalf("Contents length = %d, want 1", len(decoded.Contents))
	}
	if decoded.Contents[0].URI != result.Contents[0].URI {
		t.Errorf("Contents[0].URI = %q, want %q", decoded.Contents[0].URI, result.Contents[0].URI)
	}
	if decoded.Contents[0].Text != result.Contents[0].Text {
		t.Errorf("Contents[0].Text = %q, want %q", decoded.Contents[0].Text, result.Contents[0].Text)
	}
}

func TestJSONSchema_NestedProperties(t *testing.T) {
	t.Parallel()

	schema := JSONSchema{
		Type:        "object",
		Description: "Root schema",
		Properties: map[string]JSONSchema{
			"nested": {
				Type:        "object",
				Description: "Nested object",
				Properties: map[string]JSONSchema{
					"value": {
						Type:    "string",
						Default: "default_value",
					},
				},
			},
			"array_field": {
				Type: "array",
				Items: &JSONSchema{
					Type: "string",
					Enum: []string{"option1", "option2"},
				},
			},
		},
		Required: []string{"nested"},
	}

	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded JSONSchema
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Type != "object" {
		t.Errorf("Type = %q, want %q", decoded.Type, "object")
	}
	if _, ok := decoded.Properties["nested"]; !ok {
		t.Error("Properties['nested'] not found")
	}
	if decoded.Properties["array_field"].Items == nil {
		t.Error("Properties['array_field'].Items is nil")
	}
	if len(decoded.Properties["array_field"].Items.Enum) != 2 {
		t.Errorf("array_field.Items.Enum length = %d, want 2", len(decoded.Properties["array_field"].Items.Enum))
	}
}

func TestPingResult_JSONSerialization(t *testing.T) {
	t.Parallel()

	result := PingResult{}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// PingResult is an empty struct, should serialize to {}
	if string(data) != "{}" {
		t.Errorf("PingResult JSON = %q, want %q", string(data), "{}")
	}

	var decoded PingResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
}

func TestJSONRPCVersion(t *testing.T) {
	t.Parallel()

	if JSONRPCVersion != "2.0" {
		t.Errorf("JSONRPCVersion = %q, want %q", JSONRPCVersion, "2.0")
	}
}
