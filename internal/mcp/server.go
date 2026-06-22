// Package mcp eksponerer ENK Regnskap over Model Context Protocol slik at en
// AI-agent kan betjene appen (legge til inntekt/utgift, sporre, lage rapporter,
// angre). Serveren er en tynn adapter over internal/core, så agentens
// endringer går gjennom samme revisjonsspor og live-oppdatering (SSE) som
// nettgrensesnittet.
//
// To transporter støttes:
//   - HTTP: POST /mcp på den kjorende web-serveren (in-process => live UI).
//   - stdio: `enk-regnskap --mcp` for klienter som Claude Code/Desktop.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/kkollsga/enk-regnskap/internal/core"
)

const protocolVersion = "2024-11-05"

// Server er en transport-agnostisk MCP-server over en core.App.
type Server struct {
	app    *core.App
	tools  []Tool
	byName map[string]Tool
}

// New lager en MCP-server.
func New(app *core.App) *Server {
	s := &Server{app: app, byName: map[string]Tool{}}
	s.tools = s.buildTools()
	for _, t := range s.tools {
		s.byName[t.Name] = t
	}
	return s
}

// --- JSON-RPC 2.0 ---

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// handle behandler en enkelt JSON-RPC-melding. Returnerer nil for
// notifikasjoner (ingen respons).
func (s *Server) handle(ctx context.Context, req rpcRequest) *rpcResponse {
	resp := &rpcResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "enk-regnskap", "version": "1.0.0"},
		}
	case "notifications/initialized", "notifications/cancelled":
		return nil // notifikasjon
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		resp.Result = map[string]any{"tools": s.toolSchemas()}
	case "tools/call":
		resp.Result, resp.Error = s.callTool(ctx, req.Params)
	default:
		resp.Error = &rpcError{Code: -32601, Message: "ukjent metode: " + req.Method}
	}
	if req.ID == nil {
		return nil // ingen ID => notifikasjon, ikke svar
	}
	return resp
}

// callTool kjorer et verktoy og pakker resultatet som MCP-innhold.
func (s *Server) callTool(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: -32602, Message: "ugyldige parametre: " + err.Error()}
	}
	tool, ok := s.byName[p.Name]
	if !ok {
		return nil, &rpcError{Code: -32602, Message: "ukjent verktoy: " + p.Name}
	}
	text, err := tool.Run(ctx, Args(p.Arguments))
	if err != nil {
		// Verktoyfeil rapporteres som innhold med isError, ikke protokollfeil.
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": "Feil: " + err.Error()}},
			"isError": true,
		}, nil
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": false,
	}, nil
}

func (s *Server) toolSchemas() []map[string]any {
	out := make([]map[string]any, 0, len(s.tools))
	for _, t := range s.tools {
		out = append(out, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
		})
	}
	return out
}

// HTTPHandler returnerer en http.HandlerFunc for POST /mcp.
func (s *Server) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "kun POST", http.StatusMethodNotAllowed)
			return
		}
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeRPCError(w, "parse error: "+err.Error())
			return
		}
		resp := s.handle(r.Context(), req)
		w.Header().Set("Content-Type", "application/json")
		if resp == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func writeRPCError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rpcResponse{
		JSONRPC: "2.0",
		Error:   &rpcError{Code: -32700, Message: msg},
	})
}

// ServeStdio kjorer serveren over linje-avgrensede JSON-RPC-meldinger (stdio).
func (s *Server) ServeStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)
	enc := json.NewEncoder(out)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			_ = enc.Encode(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: err.Error()}})
			continue
		}
		resp := s.handle(ctx, req)
		if resp != nil {
			if err := enc.Encode(resp); err != nil {
				return fmt.Errorf("skriv svar: %w", err)
			}
		}
	}
	return scanner.Err()
}
