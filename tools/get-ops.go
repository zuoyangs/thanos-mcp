package tools

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// handleGetClusterInfo 获取集群综合信息
func (h *Handler) handleGetClusterInfo(ctx context.Context, arguments map[string]interface{}) (map[string]interface{}, error) {
	info := map[string]interface{}{
		"thanos_endpoint": h.Server.ThanosClient.Endpoint,
		"thanos_timeout":  h.Server.ThanosClient.Timeout.String(),
		"transport":       h.Server.Transport,
		"auth_enabled":    h.Server.Auth.Enabled,
		"auth_users":      len(h.Server.Auth.Users),
		"server_time":     time.Now().Format(time.RFC3339),
		"server_time_utc": time.Now().UTC().Format(time.RFC3339),
		"server_unix":     time.Now().Unix(),
	}

	// 尝试获取运行时信息
	rtResult, err := h.Server.ThanosClient.RuntimeInfo(ctx)
	if err == nil && rtResult.Status == "success" {
		info["runtime_info"] = rtResult.Data
	}

	// 尝试获取 flags
	flagsResult, err := h.Server.ThanosClient.Flags(ctx)
	if err == nil && flagsResult.Status == "success" {
		info["flags"] = flagsResult.Data
	}

	return successResult(info), nil
}

// handleGetStatus 获取运行状态
func (h *Handler) handleGetStatus(ctx context.Context, arguments map[string]interface{}) (map[string]interface{}, error) {
	output := map[string]interface{}{}

	// 运行时信息: start time, working directory, goroutines, GOMAXPROCS, GOGC, GODEBUG
	rtResult, err := h.Server.ThanosClient.RuntimeInfo(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("Error fetching runtime info: %v", err)), nil
	}
	if rtResult.Status != "success" {
		return errorResult(fmt.Sprintf("Runtime info failed: %s", rtResult.Error)), nil
	}
	output["runtime_information"] = rtResult.Data

	// 构建信息: version, revision, branch, buildUser, buildDate, goVersion
	buildResult, err := h.Server.ThanosClient.BuildInfo(ctx)
	if err == nil && buildResult.Status == "success" {
		output["build_information"] = buildResult.Data
	} else if err != nil {
		output["build_information_error"] = err.Error()
	}

	return successResult(output), nil
}

// handleGetStores 获取 Thanos StoreAPI 组件列表
func (h *Handler) handleGetStores(ctx context.Context, arguments map[string]interface{}) (map[string]interface{}, error) {
	storeType := getStringArg(arguments, "type") // sidecar, store, rule, receive, etc.

	result, err := h.Server.ThanosClient.Stores(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("Error fetching stores: %v", err)), nil
	}
	if result.Status != "success" {
		return errorResult(fmt.Sprintf("Stores query failed: %s", result.Error)), nil
	}

	// /api/v1/stores 返回的 data 通常是一个 store 列表或按类型分组的 map
	// 尝试按类型过滤和统计
	output := map[string]interface{}{}

	// 收集所有 store 的 external_labels 映射关系
	// key: bi, value: map[string]string 包含 cluster, location, kube_cluster_alias 等
	labelsetsMapping := map[string]map[string]string{}

	switch data := result.Data.(type) {
	case []interface{}:
		stores := data
		if storeType != "" {
			var filtered []interface{}
			for _, s := range stores {
				if m, ok := s.(map[string]interface{}); ok {
					if t, _ := m["storeType"].(string); strings.EqualFold(t, storeType) {
						filtered = append(filtered, s)
					}
				}
			}
			stores = filtered
			output["filter_type"] = storeType
		}
		output["stores"] = stores
		output["total"] = len(stores)

		// 按 storeType 统计
		typeCounts := map[string]int{}
		healthCounts := map[string]int{"healthy": 0, "unhealthy": 0}
		for _, s := range stores {
			if m, ok := s.(map[string]interface{}); ok {
				if t, _ := m["storeType"].(string); t != "" {
					typeCounts[t]++
				}
				// 检查 lastError 判断健康
				if lastErr, _ := m["lastError"].(string); lastErr != "" {
					healthCounts["unhealthy"]++
				} else {
					healthCounts["healthy"]++
				}

				// 提取 external_labels 映射关系
				if labels, ok := m["labels"].(map[string]interface{}); ok {
					extractLabelsetMapping(labels, labelsetsMapping)
				}
			}
		}
		output["type_summary"] = typeCounts
		output["health_summary"] = healthCounts

	case map[string]interface{}:
		// 有些 Thanos 版本按类型分组返回
		if storeType != "" {
			storeTypeLower := strings.ToLower(storeType)
			filtered := map[string]interface{}{}
			for k, v := range data {
				if strings.EqualFold(k, storeTypeLower) || strings.Contains(strings.ToLower(k), storeTypeLower) {
					filtered[k] = v
				}
			}
			output["stores"] = filtered
			output["filter_type"] = storeType
		} else {
			output["stores"] = data
		}
		// 统计各类型数量
		typeCounts := map[string]int{}
		for k, v := range data {
			if arr, ok := v.([]interface{}); ok {
				typeCounts[k] = len(arr)
				// 提取每个 store 的 external_labels
				for _, s := range arr {
					if m, ok := s.(map[string]interface{}); ok {
						if labels, ok := m["labels"].(map[string]interface{}); ok {
							extractLabelsetMapping(labels, labelsetsMapping)
						}
					}
				}
			}
		}
		if len(typeCounts) > 0 {
			output["type_summary"] = typeCounts
		}

	default:
		// 原样返回
		output["stores"] = result.Data
	}

	// 添加 cluster ↔ bi ↔ location 映射表
	if len(labelsetsMapping) > 0 {
		output["labelsets_mapping"] = labelsetsMapping
	}

	return successResult(output), nil
}

// extractLabelsetMapping 从 store 的 labels 中提取映射关系
// 以 bi 为主键，收集 cluster, location, kube_cluster_alias, kube_cluster_name 等字段
func extractLabelsetMapping(labels map[string]interface{}, mapping map[string]map[string]string) {
	bi, _ := labels["bi"].(string)
	cluster, _ := labels["cluster"].(string)

	// 必须有 bi 或 cluster 才有意义
	if bi == "" && cluster == "" {
		return
	}

	// 如果没有 bi，用 cluster 作为 key
	key := bi
	if key == "" {
		key = cluster
	}

	// 收集所有相关标签
	info := map[string]string{}
	for _, k := range []string{
		"bi", "cluster", "location", "kube_cluster_alias", "kube_cluster_name",
		"cluster_alias", "prometheus", "prometheus_replica",
	} {
		if v, ok := labels[k].(string); ok && v != "" {
			info[k] = v
		}
	}

	// 合并已存在的映射（可能有多个 replica，但 external_labels 应该相同）
	if existing, ok := mapping[key]; ok {
		for k, v := range info {
			existing[k] = v
		}
	} else {
		mapping[key] = info
	}
}
