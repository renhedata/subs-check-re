// services/checker/export.go
package checker

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	settingssvc "subs-check-re/services/settings"
)

// Exporter renders the export response for a given (subscriptionID, userID)
// pair. Each adapter knows which loader to call (proxies vs. server-addresses)
// and how to format the output. Adapters live in this file because each is
// small enough that splitting them across files would be more noise than locality.
type Exporter interface {
	ContentType() string
	// Filename returns the file name for Content-Disposition, or "" to skip the header.
	Filename(opts ExportOpts) string
	// Export loads the necessary data and writes the response body.
	Export(ctx context.Context, w http.ResponseWriter, subscriptionID, userID string, opts ExportOpts) error
}

// ExportOpts carries target-specific knobs (e.g. RouterOS list name) plus
// metadata that exporters need to log the request.
type ExportOpts struct {
	ListName string
	ClientIP string
}

// exporters is the dispatch table. New formats register here.
var exporters = map[string]Exporter{
	"clash":    clashExporter{},
	"base64":   base64Exporter{},
	"routeros": routerOSExporter{},
}

// Export generates a subscription link from the latest completed check results.
// If subscriptionID is "all", combines nodes from all subscriptions.
//
// Supported targets: clash (default), base64, routeros
// For routeros target, use ?list=<address-list-name> (default: clash_servers)
//
//encore:api public raw method=GET path=/export/:subscriptionID
func Export(w http.ResponseWriter, req *http.Request) {
	parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	if len(parts) < 2 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	dispatchExport(w, req, parts[1])
}

// ExportAll is a testable wrapper around the export-all path.
func ExportAll(w http.ResponseWriter, req *http.Request) {
	dispatchExport(w, req, "all")
}

func dispatchExport(w http.ResponseWriter, req *http.Request, subscriptionID string) {
	ctx := req.Context()

	token := req.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "token required", http.StatusUnauthorized)
		return
	}
	userResp, err := settingssvc.GetUserIDByAPIKey(ctx, token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	target := req.URL.Query().Get("target")
	if target == "" {
		target = "clash"
	}
	exporter, ok := exporters[target]
	if !ok {
		exporter = exporters["clash"]
	}

	opts := ExportOpts{
		ListName: req.URL.Query().Get("list"),
		ClientIP: clientIP(req),
	}
	if opts.ListName == "" {
		opts.ListName = "clash_servers"
	}

	w.Header().Set("Content-Type", exporter.ContentType())
	if fn := exporter.Filename(opts); fn != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fn))
	}

	if err := exporter.Export(ctx, w, subscriptionID, userResp.UserID, opts); err != nil {
		// Headers may already be flushed by Render; best-effort error reporting.
		_, _ = fmt.Fprintf(w, "\n# export error: %v\n", err)
	}
}

// --- Adapters ---

type clashExporter struct{}

func (clashExporter) ContentType() string          { return "text/yaml; charset=utf-8" }
func (clashExporter) Filename(_ ExportOpts) string { return "" }
func (clashExporter) Export(ctx context.Context, w http.ResponseWriter, subID, userID string, opts ExportOpts) error {
	proxies, err := loadExportProxies(ctx, subID, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return nil
	}
	logExport(ctx, subID, userID, opts.ClientIP)
	return renderClash(w, proxies)
}

type base64Exporter struct{}

func (base64Exporter) ContentType() string          { return "text/plain; charset=utf-8" }
func (base64Exporter) Filename(_ ExportOpts) string { return "" }
func (base64Exporter) Export(ctx context.Context, w http.ResponseWriter, subID, userID string, opts ExportOpts) error {
	proxies, err := loadExportProxies(ctx, subID, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return nil
	}
	logExport(ctx, subID, userID, opts.ClientIP)
	return renderBase64(w, proxies)
}

type routerOSExporter struct{}

func (routerOSExporter) ContentType() string { return "text/plain; charset=utf-8" }
func (routerOSExporter) Filename(opts ExportOpts) string {
	return opts.ListName + ".rsc"
}
func (routerOSExporter) Export(ctx context.Context, w http.ResponseWriter, subID, userID string, opts ExportOpts) error {
	servers, notFound, err := latestServerAddresses(ctx, subID, userID)
	if notFound {
		http.Error(w, "subscription not found", http.StatusNotFound)
		return nil
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return nil
	}
	return renderRouterOS(w, servers, opts.ListName)
}

// loadExportProxies routes single-vs-all to the right loader.
func loadExportProxies(ctx context.Context, subID, userID string) ([]map[string]any, error) {
	if subID == "all" {
		return latestUsableProxiesAcrossAllSubs(ctx, userID)
	}
	return latestUsableProxies(ctx, subID, userID)
}

