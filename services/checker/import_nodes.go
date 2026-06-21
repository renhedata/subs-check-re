// services/checker/import_nodes.go
package checker

import (
	"context"
	"fmt"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"

	authsvc "subs-check-re/services/auth"
	subsvc "subs-check-re/services/subscription"
)

// ImportNodesParams is the request body for ImportNodes.
type ImportNodesParams struct {
	// Content is a subscription payload pasted by the user: Clash YAML
	// (proxies:) or a V2Ray base64 blob / share links — same formats the URL
	// fetcher accepts.
	Content string `json:"content"`
}

// ImportNodesResponse reports how many nodes were imported.
type ImportNodesResponse struct {
	Count int `json:"count"`
}

// ImportNodes replaces a subscription's node list from pasted content instead of
// a URL fetch. Lets users populate subscriptions whose URL is unreachable from
// the server. The imported list persists until the next manual import or
// refresh — checks never overwrite it.
//
//encore:api auth method=POST path=/subscription/:subscriptionID/import-nodes
func ImportNodes(ctx context.Context, subscriptionID string, p *ImportNodesParams) (*ImportNodesResponse, error) {
	_ = encauth.Data().(*authsvc.UserClaims)

	// Ownership: GetSubscription filters by the caller's user_id.
	if _, err := subsvc.GetSubscription(ctx, subscriptionID); err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("subscription not found").Err()
	}

	count, err := importNodes(ctx, subscriptionID, p.Content)
	if err != nil {
		return nil, errs.B().Code(errs.InvalidArgument).Msg(err.Error()).Err()
	}
	return &ImportNodesResponse{Count: count}, nil
}

// importNodes parses pasted content and replaces the subscription's nodes.
// Separated from the endpoint so it can be tested without seeding the
// subscription service.
func importNodes(ctx context.Context, subscriptionID, content string) (int, error) {
	proxies, err := parseProxies([]byte(content))
	if err != nil {
		return 0, fmt.Errorf("could not parse nodes: %w", err)
	}
	if len(proxies) == 0 {
		return 0, fmt.Errorf("no nodes found in the pasted content")
	}
	nodeIDs, err := defaultJobStore.replaceNodes(ctx, subscriptionID, proxies)
	if err != nil {
		return 0, fmt.Errorf("failed to save nodes: %w", err)
	}
	return len(nodeIDs), nil
}
