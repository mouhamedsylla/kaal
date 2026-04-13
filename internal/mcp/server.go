package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mouhamedsylla/pilot/internal/adapters/secrets/local"
)

// Server is the MCP server — JSON-RPC 2.0 over stdio.
type Server struct {
	tools    map[string]Tool
	handlers map[string]HandlerFunc
}

// Tool describes an MCP tool exposed to AI clients.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema follows JSON Schema (draft-07 subset).
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property describes a single input parameter.
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

// HandlerFunc processes a tool call and returns a result or an error.
type HandlerFunc func(ctx context.Context, params map[string]any) (any, error)

// request is an incoming JSON-RPC 2.0 message.
type request struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params"`
}

// response is an outgoing JSON-RPC 2.0 message.
type response struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`          // always present — null for parse errors
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewServer() *Server {
	s := &Server{
		tools:    make(map[string]Tool),
		handlers: make(map[string]HandlerFunc),
	}
	loadEnvLocal()
	s.registerAll()
	return s
}

// loadEnvLocal loads persisted credentials from .env.local into the process
// environment at server startup, so credentials set in previous sessions are
// immediately available without asking the user again.
func loadEnvLocal() {
	vars, err := local.ListFile(".env.local")
	if err != nil {
		return // file doesn't exist yet — no-op
	}
	for k, v := range vars {
		if os.Getenv(k) == "" { // don't override existing env vars
			os.Setenv(k, v)
		}
	}
}

// Register adds a tool and its handler to the server.
func (s *Server) Register(tool Tool, handler HandlerFunc) {
	s.tools[tool.Name] = tool
	s.handlers[tool.Name] = handler
}

// Serve reads JSON-RPC requests from stdin and writes responses to stdout.
func (s *Server) Serve(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeError(nil, -32700, "parse error")
			continue
		}

		// JSON-RPC 2.0: notifications have no "id" — never respond to them.
		if req.ID == nil && req.Method != "" {
			continue
		}

		resp := s.dispatch(ctx, &req)
		if resp != nil {
			s.write(resp)
		}
	}
	return scanner.Err()
}

func (s *Server) dispatch(ctx context.Context, req *request) *response {
	switch req.Method {
	case "initialize":
		return &response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "pilot", "version": "0.1.0"},
			},
		}

	case "ping":
		return &response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{},
		}

	case "tools/list":
		tools := make([]Tool, 0, len(s.tools))
		for _, t := range s.tools {
			tools = append(tools, t)
		}
		return &response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{"tools": tools},
		}

	case "tools/call":
		toolName, _ := req.Params["name"].(string)
		args, _ := req.Params["arguments"].(map[string]any)

		handler, ok := s.handlers[toolName]
		if !ok {
			return &response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &rpcError{Code: -32601, Message: fmt.Sprintf("tool %q not found", toolName)},
			}
		}

		result, err := handler(ctx, args)
		if err != nil {
			return &response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"content": []map[string]any{
						{"type": "text", "text": err.Error()},
					},
					"isError": true,
				},
			}
		}

		text, _ := json.Marshal(result)
		return &response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": string(text)},
				},
			},
		}

	default:
		return &response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: fmt.Sprintf("method %q not found", req.Method)},
		}
	}
}

func (s *Server) write(resp *response) {
	data, _ := json.Marshal(resp)
	fmt.Fprintf(os.Stdout, "%s\n", data)
}

func (s *Server) writeError(id any, code int, msg string) {
	s.write(&response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: msg},
	})
}

// ensure Serve consumes io.Reader without unused import
var _ io.Reader = os.Stdin
