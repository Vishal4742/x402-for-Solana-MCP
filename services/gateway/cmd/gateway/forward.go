package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxUpstreamBody caps how much of an upstream response the gateway will accept.
const maxUpstreamBody = 1 << 20

// forwardToUpstream POSTs {"tool":...,"input":...} to {upstream}/mcp and returns the
// upstream tool result as a JSON object. Any transport error, non-2xx status,
// oversized body, or non-JSON body is reported as an error so callers can surface
// upstream_error.
func (a *app) forwardToUpstream(ctx context.Context, forwarding serverForwarding, serverID, requestID string, body any) (map[string]any, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(forwarding.UpstreamURL, "/")+"/mcp", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gateway-Server-Id", serverID)
	req.Header.Set("X-Gateway-Request-Id", requestID)
	req.Header.Set("X-Gateway-Key", forwarding.APIKey)

	resp, err := a.upstreamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamBody+1))
	if err != nil {
		return nil, fmt.Errorf("reading upstream response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream returned %s: %s", resp.Status, truncateForLog(data, 256))
	}
	if len(data) > maxUpstreamBody {
		return nil, fmt.Errorf("upstream response exceeds %d bytes", maxUpstreamBody)
	}

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("upstream returned invalid JSON: %w", err)
	}
	if obj, ok := value.(map[string]any); ok {
		return obj, nil
	}
	return map[string]any{"result": value}, nil
}

func truncateForLog(data []byte, limit int) string {
	text := strings.TrimSpace(string(data))
	if len(text) > limit {
		return text[:limit] + "..."
	}
	return text
}
