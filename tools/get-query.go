package tools

import (
	"context"
	"fmt"
	"strings"
)

// ValidateLabelFilter checks if query contains required bi or cluster or kube_cluster_alias label
func ValidateLabelFilter(query string) error {
	queryLower := strings.ToLower(query)
	if !strings.Contains(queryLower, "bi=") && !strings.Contains(queryLower, "cluster=") && !strings.Contains(queryLower, "kube_cluster_alias=") {
		return fmt.Errorf("query must contain 'bi=' or 'cluster=' or 'kube_cluster_alias=' label filter. For example: 'up{cluster=\"prod\"}' or 'metric{bi=\"analytics\"}' or 'metric{kube_cluster_alias=\"prod-k8s\"}'")
	}
	return nil
}

// handleQuery 处理即时查询
func (h *Handler) handleQuery(ctx context.Context, arguments map[string]interface{}) (map[string]interface{}, error) {
	query := getStringArg(arguments, "query")
	if query == "" {
		return errorResult("Error: 'query' parameter is required and must be a string"), nil
	}
	h.Server.logEndpoint()

	if err := ValidateLabelFilter(query); err != nil {
		return errorResult(fmt.Sprintf("Validation Error: %v", err)), nil
	}

	result, err := h.Server.ThanosClient.Query(ctx, query)
	if err != nil {
		return errorResult(fmt.Sprintf("Error executing query: %v", err)), nil
	}
	if result.Status != "success" {
		return errorResult(fmt.Sprintf("Query failed: %s (type: %s)", result.Error, result.ErrorType)), nil
	}
	return successResult(result.Data), nil
}

// handleQueryRange 处理范围查询
func (h *Handler) handleQueryRange(ctx context.Context, arguments map[string]interface{}) (map[string]interface{}, error) {
	query := getStringArg(arguments, "query")
	if query == "" {
		return errorResult("Error: 'query' parameter is required"), nil
	}

	start, ok := arguments["start"].(float64)
	if !ok {
		return errorResult("Error: 'start' parameter is required and must be a Unix timestamp"), nil
	}
	end, ok := arguments["end"].(float64)
	if !ok {
		return errorResult("Error: 'end' parameter is required and must be a Unix timestamp"), nil
	}
	step := getStringArg(arguments, "step")
	if step == "" {
		return errorResult("Error: 'step' parameter is required (e.g., '15s', '1m', '5m', '1h')"), nil
	}

	if err := ValidateLabelFilter(query); err != nil {
		return errorResult(fmt.Sprintf("Validation Error: %v", err)), nil
	}

	result, err := h.Server.ThanosClient.RangeQuery(ctx, query, int64(start), int64(end), step)
	if err != nil {
		return errorResult(fmt.Sprintf("Error executing range query: %v", err)), nil
	}
	if result.Status != "success" {
		return errorResult(fmt.Sprintf("Range query failed: %s (type: %s)", result.Error, result.ErrorType)), nil
	}
	return successResult(result.Data), nil
}
