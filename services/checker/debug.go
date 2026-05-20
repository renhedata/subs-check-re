package checker

type DebugStep struct {
	Type        string         `json:"type"`
	Description string         `json:"description"`
	Details     map[string]any `json:"details"`
}

type DebugTrace struct {
	Platform string      `json:"platform"`
	Result   bool        `json:"result"`
	Steps    []DebugStep `json:"steps"`
}

type NodeDebug struct {
	NodeID   string       `json:"node_id"`
	NodeName string       `json:"node_name"`
	Traces   []DebugTrace `json:"traces"`
}
