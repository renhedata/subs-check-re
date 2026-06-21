// services/checker/refresh.go
package checker

import (
	"context"

	encauth "encore.dev/beta/auth"
	"encore.dev/beta/errs"

	authsvc "subs-check-re/services/auth"
	subsvc "subs-check-re/services/subscription"
)

// RefreshResponse reports how many nodes a refresh produced.
type RefreshResponse struct {
	Count int `json:"count"`
}

// RefreshSubscription re-fetches the subscription URL and replaces its node
// list. This is the explicit, manual counterpart to the old per-check fetch:
// nodes only change when the user asks for it.
//
//encore:api auth method=POST path=/subscription/:subscriptionID/refresh
func RefreshSubscription(ctx context.Context, subscriptionID string) (*RefreshResponse, error) {
	_ = encauth.Data().(*authsvc.UserClaims)

	sub, err := subsvc.GetSubscription(ctx, subscriptionID)
	if err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("subscription not found").Err()
	}
	if sub.URL == "" {
		return nil, errs.B().Code(errs.FailedPrecondition).Msg("subscription has no URL; import nodes instead").Err()
	}

	count, err := refreshNodes(ctx, defaultFetcher, subscriptionID, sub.URL)
	if err != nil {
		return nil, errs.B().Code(errs.Unavailable).Msgf("could not fetch subscription: %v", err).Err()
	}
	return &RefreshResponse{Count: count}, nil
}

// TestFetchResponse reports whether a dry-run fetch succeeded. A failure is
// returned in-band (HTTP 200, ok=false, error set) so the UI can show the exact
// reason instead of a generic request error.
type TestFetchResponse struct {
	Ok    bool   `json:"ok"`
	Count int    `json:"count"`
	Error string `json:"error,omitempty"`
}

// TestFetch tries to fetch the subscription URL without persisting anything, so
// the user can check whether the server can reach the provider and how many
// nodes it would get.
//
//encore:api auth method=POST path=/subscription/:subscriptionID/test-fetch
func TestFetch(ctx context.Context, subscriptionID string) (*TestFetchResponse, error) {
	_ = encauth.Data().(*authsvc.UserClaims)

	sub, err := subsvc.GetSubscription(ctx, subscriptionID)
	if err != nil {
		return nil, errs.B().Code(errs.NotFound).Msg("subscription not found").Err()
	}
	if sub.URL == "" {
		return &TestFetchResponse{Ok: false, Error: "subscription has no URL configured"}, nil
	}

	count, err := dryRunFetch(ctx, defaultFetcher, sub.URL)
	if err != nil {
		return &TestFetchResponse{Ok: false, Error: err.Error()}, nil
	}
	return &TestFetchResponse{Ok: true, Count: count}, nil
}

// refreshNodes fetches the URL and replaces the subscription's nodes. Takes the
// fetcher explicitly so tests can drive it without real network I/O.
func refreshNodes(ctx context.Context, fetcher SubscriptionFetcher, subscriptionID, url string) (int, error) {
	proxies, err := fetchWithRetry(ctx, fetcher, url)
	if err != nil {
		return 0, err
	}
	nodeIDs, err := defaultJobStore.replaceNodes(ctx, subscriptionID, proxies)
	if err != nil {
		return 0, err
	}
	return len(nodeIDs), nil
}

// dryRunFetch fetches the URL and reports the node count without persisting.
func dryRunFetch(ctx context.Context, fetcher SubscriptionFetcher, url string) (int, error) {
	proxies, err := fetchWithRetry(ctx, fetcher, url)
	if err != nil {
		return 0, err
	}
	return len(proxies), nil
}
