package checker

import "encoding/json"

// DebugStep captures one step in a platform check trace.
type DebugStep struct {
	Type        string          `json:"type"`        // "http_request" | "http_response" | "variable" | "condition" | "log" | "error"
	Description string          `json:"description"` // human-readable summary
	Details     json.RawMessage `json:"details"`     // free-form key-value pairs as JSON
}

// DebugTrace is the full trace for one platform check on one node.
type DebugTrace struct {
	Platform string      `json:"platform"`
	Result   bool        `json:"result"`
	Steps    []DebugStep `json:"steps"`
}

// NodeDebug holds debug traces for all platforms checked on one node.
type NodeDebug struct {
	NodeID   string       `json:"node_id"`
	NodeName string       `json:"node_name"`
	Traces   []DebugTrace `json:"traces"`
}
