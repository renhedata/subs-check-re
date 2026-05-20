package checker

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"

	authsvc "subs-check-re/services/auth"
)

// builtinKeys are platform keys with dedicated bool columns in check_results.
var builtinKeys = map[string]bool{
	"netflix":         true,
	"youtube":         true,
	"youtube_premium": true,
	"openai":          true,
	"claude":          true,
	"gemini":          true,
	"grok":            true,
	"disney":          true,
	"tiktok":          true,
}

// ConditionDef defines an HTTP-based condition check.
type ConditionDef struct {
	URL                 string            `json:"url"`
	Method              string            `json:"method,omitempty"`
	Headers             map[string]string `json:"headers,omitempty"`
	StatusCode          int               `json:"status_code,omitempty"`
	BodyContains        []string          `json:"body_contains,omitempty"`
	BodyContainsAny     []string          `json:"body_contains_any,omitempty"`
	BodyNotContains     []string          `json:"body_not_contains,omitempty"`
	FinalURLContains    string            `json:"final_url_contains,omitempty"`
	FinalURLNotContains string            `json:"final_url_not_contains,omitempty"`
}

// ScriptDef holds the script source code for tengo/lua/js/ts rule types.
// Prelude is optional user-defined shared code (helpers, constants) injected before Code.
type ScriptDef struct {
	Prelude string `json:"prelude,omitempty"`
	Code    string `json:"code"`
}

// PlatformRule is a user-defined platform detection rule.
type PlatformRule struct {
	ID         string          `json:"id"`
	UserID     string          `json:"user_id"`
	Name       string          `json:"name"`
	Key        string          `json:"key"`
	Icon       string          `json:"icon"`
	Enabled    bool            `json:"enabled"`
	RuleType   string          `json:"rule_type"`
	Definition json.RawMessage `json:"definition"`
	IsDefault  bool            `json:"is_default"`
	SortOrder  int             `json:"sort_order"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// ListRulesResponse is returned by GET /platform-rules.
type ListRulesResponse struct {
	Rules []*PlatformRule `json:"rules"`
}

// CreateRuleParams is the request body for POST /platform-rules.
type CreateRuleParams struct {
	Name       string          `json:"name"`
	Key        string          `json:"key"`
	Icon       string          `json:"icon"`
	Enabled    bool            `json:"enabled"`
	RuleType   string          `json:"rule_type"`
	Definition json.RawMessage `json:"definition"`
	SortOrder  int             `json:"sort_order"`
}

// UpdateRuleParams is the request body for PUT /platform-rules/:ruleId.
type UpdateRuleParams struct {
	Name       string          `json:"name"`
	Icon       string          `json:"icon"`
	Enabled    bool            `json:"enabled"`
	RuleType   string          `json:"rule_type"`
	Definition json.RawMessage `json:"definition"`
	SortOrder  int             `json:"sort_order"`
}

// TestRuleParams is the request body for POST /platform-rules/test.
type TestRuleParams struct {
	RuleType   string          `json:"rule_type"`
	Definition json.RawMessage `json:"definition"`
	NodeID     string          `json:"node_id,omitempty"`
}

// TestRuleResult is returned by POST /platform-rules/test.
type TestRuleResult struct {
	OK              bool              `json:"ok"`
	Error           string            `json:"error,omitempty"`
	StatusCode      int               `json:"status_code,omitempty"`
	FinalURL        string            `json:"final_url,omitempty"`
	Body            string            `json:"body,omitempty"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
	NodeName        string            `json:"node_name,omitempty"`
	DurationMs      int64             `json:"duration_ms,omitempty"`
}

// NodeSummary is a minimal proxy node entry for the test node picker.
type NodeSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// ListTestNodesResponse is returned by GET /platform-rules/test-nodes.
type ListTestNodesResponse struct {
	Nodes []*NodeSummary `json:"nodes"`
}

var validRuleTypes = map[string]bool{
	"condition": true,
	"js":        true,
	"ts":        true,
	"tengo":     true,
	"lua":       true,
}

// ListRules returns all platform rules for the current user.
// Seeds default rules on first call if none exist.
//
//encore:api auth method=GET path=/platform-rules
func ListRules(ctx context.Context) (*ListRulesResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)
	userID := claims.UserID

	rules, err := loadUserRules(ctx, userID)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to load rules").Err()
	}
	if len(rules) == 0 {
		if seedErr := seedDefaultRules(ctx, userID); seedErr != nil {
			return nil, errs.B().Code(errs.Internal).Msg("failed to seed default rules").Err()
		}
		rules, err = loadUserRules(ctx, userID)
		if err != nil {
			return nil, errs.B().Code(errs.Internal).Msg("failed to load rules after seed").Err()
		}
	}
	return &ListRulesResponse{Rules: rules}, nil
}

// CreateRule creates a new platform rule for the current user.
//
//encore:api auth method=POST path=/platform-rules
func CreateRule(ctx context.Context, p *CreateRuleParams) (*PlatformRule, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Key) == "" {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("name and key are required").Err()
	}
	if !validRuleTypes[p.RuleType] {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("invalid rule_type").Err()
	}

	id := uuid.New().String()
	now := time.Now()
	defJSON, _ := json.Marshal(p.Definition)

	if _, err := db.Exec(ctx, `
		INSERT INTO platform_rules (id, user_id, name, key, icon, enabled, rule_type, definition, is_default, sort_order, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,false,$9,$10,$10)
	`, id, claims.UserID, p.Name, p.Key, p.Icon, p.Enabled, p.RuleType, defJSON, p.SortOrder, now); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to create rule").Err()
	}

	return &PlatformRule{
		ID:         id,
		UserID:     claims.UserID,
		Name:       p.Name,
		Key:        p.Key,
		Icon:       p.Icon,
		Enabled:    p.Enabled,
		RuleType:   p.RuleType,
		Definition: p.Definition,
		IsDefault:  false,
		SortOrder:  p.SortOrder,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// UpdateRule updates a platform rule owned by the current user.
//
//encore:api auth method=PUT path=/platform-rules/:ruleId
func UpdateRule(ctx context.Context, ruleId string, p *UpdateRuleParams) (*PlatformRule, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	if !validRuleTypes[p.RuleType] {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("invalid rule_type").Err()
	}

	now := time.Now()
	defJSON, _ := json.Marshal(p.Definition)

	result, err := db.Exec(ctx, `
		UPDATE platform_rules
		SET name=$3, icon=$4, enabled=$5, rule_type=$6, definition=$7, sort_order=$8, updated_at=$9
		WHERE id=$1 AND user_id=$2
	`, ruleId, claims.UserID, p.Name, p.Icon, p.Enabled, p.RuleType, defJSON, p.SortOrder, now)
	if err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to update rule").Err()
	}
	rows := result.RowsAffected()
	if rows == 0 {
		return nil, errs.B().Code(errs.NotFound).Msg("rule not found").Err()
	}

	var rule PlatformRule
	var rawDef []byte
	if err := db.QueryRow(ctx,
		`SELECT id, user_id, name, key, icon, enabled, rule_type, definition, is_default, sort_order, created_at, updated_at
		 FROM platform_rules WHERE id=$1`,
		ruleId,
	).Scan(&rule.ID, &rule.UserID, &rule.Name, &rule.Key, &rule.Icon, &rule.Enabled, &rule.RuleType,
		&rawDef, &rule.IsDefault, &rule.SortOrder, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
		return nil, errs.B().Code(errs.Internal).Msg("failed to read updated rule").Err()
	}
	rule.Definition = rawDef
	return &rule, nil
}

// DeleteRule removes a platform rule owned by the current user.
//
//encore:api auth method=DELETE path=/platform-rules/:ruleId
func DeleteRule(ctx context.Context, ruleId string) error {
	claims := encauth.Data().(*authsvc.UserClaims)

	result, err := db.Exec(ctx, `DELETE FROM platform_rules WHERE id=$1 AND user_id=$2`, ruleId, claims.UserID)
	if err != nil {
		return errs.B().Code(errs.Internal).Msg("failed to delete rule").Err()
	}
	rows := result.RowsAffected()
	if rows == 0 {
		return errs.B().Code(errs.NotFound).Msg("rule not found").Err()
	}
	return nil
}

// TestRule runs a rule definition and returns verbose debug output.
// If NodeID is set, the request is routed through that node's proxy.
//
//encore:api auth method=POST path=/platform-rules/test
func TestRule(ctx context.Context, p *TestRuleParams) (*TestRuleResult, error) {
	if !validRuleTypes[p.RuleType] {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("invalid rule_type").Err()
	}
	claims := encauth.Data().(*authsvc.UserClaims)

	var httpClient *http.Client
	var nodeName string

	if p.NodeID != "" {
		var configJSON []byte
		err := db.QueryRow(ctx, `
			SELECT n.name, n.config FROM nodes n
			WHERE n.id = $1
			  AND n.subscription_id IN (
			        SELECT DISTINCT subscription_id FROM check_jobs WHERE user_id = $2
			      )
		`, p.NodeID, claims.UserID).Scan(&nodeName, &configJSON)
		if err == nil && len(configJSON) > 0 {
			var mapping map[string]any
			if json.Unmarshal(configJSON, &mapping) == nil {
				if pc := newProxyClient(mapping); pc != nil {
					defer pc.close()
					httpClient = pc.Client
				}
			}
		}
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
		nodeName = ""
	}

	start := time.Now()

	if p.RuleType == "condition" {
		dbg, err := testConditionVerbose(ctx, httpClient, p.Definition)
		ms := time.Since(start).Milliseconds()
		if err != nil {
			return &TestRuleResult{OK: false, Error: err.Error(), DurationMs: ms, NodeName: nodeName}, nil
		}
		return &TestRuleResult{
			OK:              dbg.ok,
			StatusCode:      dbg.statusCode,
			FinalURL:        dbg.finalURL,
			Body:            dbg.body,
			ResponseHeaders: dbg.responseHeaders,
			NodeName:        nodeName,
			DurationMs:      ms,
		}, nil
	}

	rule := &PlatformRule{RuleType: p.RuleType, Definition: p.Definition}
	ok, err := runRule(ctx, httpClient, rule, nil)
	ms := time.Since(start).Milliseconds()
	if err != nil {
		return &TestRuleResult{OK: false, Error: err.Error(), DurationMs: ms, NodeName: nodeName}, nil
	}
	return &TestRuleResult{OK: ok, DurationMs: ms, NodeName: nodeName}, nil
}

// ListTestNodes returns all proxy nodes available to the current user, for the test node picker.
//
//encore:api auth method=GET path=/platform-rules/test-nodes
func ListTestNodes(ctx context.Context) (*ListTestNodesResponse, error) {
	claims := encauth.Data().(*authsvc.UserClaims)

	rows, err := db.Query(ctx, `
		SELECT DISTINCT ON (n.name) n.id, n.name, COALESCE(n.type, '')
		FROM nodes n
		WHERE n.subscription_id IN (
		    SELECT DISTINCT subscription_id FROM check_jobs WHERE user_id = $1
		)
		ORDER BY n.name
		LIMIT 500
	`, claims.UserID)
	if err != nil {
		return &ListTestNodesResponse{Nodes: []*NodeSummary{}}, nil
	}
	defer rows.Close()

	var nodes []*NodeSummary
	for rows.Next() {
		var n NodeSummary
		if err := rows.Scan(&n.ID, &n.Name, &n.Type); err == nil {
			nodes = append(nodes, &n)
		}
	}
	if nodes == nil {
		nodes = []*NodeSummary{}
	}
	return &ListTestNodesResponse{Nodes: nodes}, nil
}

// loadUserRules fetches all platform rules for a user ordered by sort_order.
func loadUserRules(ctx context.Context, userID string) ([]*PlatformRule, error) {
	rows, err := db.Query(ctx,
		`SELECT id, user_id, name, key, icon, enabled, rule_type, definition, is_default, sort_order, created_at, updated_at
		 FROM platform_rules WHERE user_id=$1 ORDER BY sort_order, created_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*PlatformRule
	for rows.Next() {
		var r PlatformRule
		var rawDef []byte
		if err := rows.Scan(&r.ID, &r.UserID, &r.Name, &r.Key, &r.Icon, &r.Enabled, &r.RuleType,
			&rawDef, &r.IsDefault, &r.SortOrder, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.Definition = rawDef
		rules = append(rules, &r)
	}
	return rules, rows.Err()
}
