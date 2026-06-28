package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Endepunktsfilen lar en lokal AI-agent finne den kjorende appens HTTP-adresse
// (og dermed MCP-endepunktet) uten konfigurasjon. Desktop-appen binder en
// tilfeldig port, så uten denne filen er den ikke mulig å nå utenfra. Filen
// ligger i basismappen og fjernes når prosessen avslutter normalt.

// MCPEndpoint beskriver hvor den kjorende appen svarer. PID + StartedAt lar en
// agent oppdage en foreldet fil (prosessen er død) hvis den ikke ble ryddet.
type MCPEndpoint struct {
	BaseURL   string `json:"base_url"` // f.eks. http://127.0.0.1:54321
	MCPURL    string `json:"mcp_url"`  // base_url + /mcp
	PID       int    `json:"pid"`
	StartedAt string `json:"started_at"` // RFC3339
}

// MCPEndpointPath er stien til endepunktsfilen for en gitt basismappe.
func MCPEndpointPath(baseDir string) string {
	if baseDir == "" {
		baseDir = DefaultBaseDir()
	}
	return filepath.Join(baseDir, ".mcp-endpoint.json")
}

// WriteMCPEndpoint skriver appens adresse til endepunktsfilen.
func WriteMCPEndpoint(baseDir, baseURL string) error {
	if baseDir == "" {
		baseDir = DefaultBaseDir()
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(MCPEndpoint{
		BaseURL:   baseURL,
		MCPURL:    baseURL + "/mcp",
		PID:       os.Getpid(),
		StartedAt: time.Now().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(MCPEndpointPath(baseDir), b, 0o644)
}

// RemoveMCPEndpoint fjerner endepunktsfilen (best effort ved avslutning).
func RemoveMCPEndpoint(baseDir string) {
	_ = os.Remove(MCPEndpointPath(baseDir))
}
