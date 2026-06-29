package tools

import (
	"context"
	"fmt"
	"strings"
)

// handleGetTargets 获取采集目标状态
func (h *Handler) handleGetTargets(ctx context.Context, arguments map[string]interface{}) (map[string]interface{}, error) {
	state := getStringArg(arguments, "state") // active, dropped, any
	scrapePool := getStringArg(arguments, "scrape_pool")
	job := getStringArg(arguments, "job")
	bi := getStringArg(arguments, "bi")
	cluster := getStringArg(arguments, "cluster")
	kubeClusterAlias := getStringArg(arguments, "kube_cluster_alias")
	clusterAlias := getStringArg(arguments, "cluster_alias")
	limit := int(getFloatArg(arguments, "limit", 0)) // 0 表示不限

	// 强制过滤保护：必须提供至少一个过滤条件，防止拉取全量数据导致超时/OOM
	if scrapePool == "" && job == "" && bi == "" && cluster == "" && kubeClusterAlias == "" && clusterAlias == "" {
		return errorResult("get_targets 拒绝全量拉取：请至少提供一个过滤参数（scrape_pool / job / bi / cluster / kube_cluster_alias / cluster_alias），避免拉取全量 Targets 导致超时或内存溢出。如需概览，请使用 limit 参数限制返回数量。"), nil
	}

	// scrape_pool 在服务端过滤（减少网络传输量），其余参数在客户端过滤
	result, err := h.Server.ThanosClient.Targets(ctx, state, scrapePool)
	if err != nil {
		return errorResult(fmt.Sprintf("Error fetching targets: %v", err)), nil
	}
	if result.Status != "success" {
		return errorResult(fmt.Sprintf("Targets query failed: %s", result.Error)), nil
	}

	if result.Data == nil {
		return successResult(map[string]interface{}{"active_targets": []interface{}{}, "dropped_targets": []interface{}{}}), nil
	}

	output := map[string]interface{}{}

	// 过滤逻辑
	var filteredActive []interface{}
	for _, t := range result.Data.ActiveTargets {
		if m, ok := t.(map[string]interface{}); ok {
			labels, _ := m["labels"].(map[string]interface{})

			// job 过滤（AND 关系，必须匹配）
			if job != "" {
				j, _ := labels["job"].(string)
				if !strings.Contains(strings.ToLower(j), strings.ToLower(job)) {
					continue
				}
			}

			// bi/cluster/kube_cluster_alias/cluster_alias 过滤（OR 关系，匹配任意一个即可）
			if bi != "" || cluster != "" || kubeClusterAlias != "" || clusterAlias != "" {
				b, _ := labels["bi"].(string)
				c, _ := labels["cluster"].(string)
				kca, _ := labels["kube_cluster_alias"].(string)
				ca, _ := labels["cluster_alias"].(string)

				matched := false
				if bi != "" && b == bi {
					matched = true
				}
				if cluster != "" && c == cluster {
					matched = true
				}
				if kubeClusterAlias != "" && kca == kubeClusterAlias {
					matched = true
				}
				if clusterAlias != "" && ca == clusterAlias {
					matched = true
				}

				if !matched {
					continue
				}
			}

			filteredActive = append(filteredActive, t)

			// limit 截断
			if limit > 0 && len(filteredActive) >= limit {
				break
			}
		}
	}

	output["active_targets"] = filteredActive
	output["active_count"] = len(filteredActive)

	// 记录过滤条件
	filters := map[string]string{}
	if scrapePool != "" {
		filters["scrape_pool"] = scrapePool
	}
	if job != "" {
		filters["job"] = job
	}
	if bi != "" {
		filters["bi"] = bi
	}
	if cluster != "" {
		filters["cluster"] = cluster
	}
	if kubeClusterAlias != "" {
		filters["kube_cluster_alias"] = kubeClusterAlias
	}
	if clusterAlias != "" {
		filters["cluster_alias"] = clusterAlias
	}
	if len(filters) > 0 {
		output["filters"] = filters
	}
	if limit > 0 {
		output["limit"] = limit
		output["truncated"] = len(result.Data.ActiveTargets) > limit
	}

	// 统计健康状态
	healthStats := map[string]int{"up": 0, "down": 0, "unknown": 0}
	for _, t := range filteredActive {
		if m, ok := t.(map[string]interface{}); ok {
			health, _ := m["health"].(string)
			switch health {
			case "up":
				healthStats["up"]++
			case "down":
				healthStats["down"]++
			default:
				healthStats["unknown"]++
			}
		}
	}
	output["health_summary"] = healthStats

	// 如果没有过滤条件，返回 dropped 统计
	if job == "" && bi == "" && cluster == "" && kubeClusterAlias == "" && clusterAlias == "" {
		output["dropped_count"] = len(result.Data.DroppedTargets)
	}

	return successResult(output), nil
}

// handleGetRules 获取告警规则和录制规则
func (h *Handler) handleGetRules(ctx context.Context, arguments map[string]interface{}) (map[string]interface{}, error) {
	ruleType := getStringArg(arguments, "type") // alert, record
	ruleGroup := getStringArg(arguments, "rule_group")
	file := getStringArg(arguments, "file")
	group := getStringArg(arguments, "group")
	bi := getStringArg(arguments, "bi")
	cluster := getStringArg(arguments, "cluster")
	kubeClusterAlias := getStringArg(arguments, "kube_cluster_alias")
	clusterAlias := getStringArg(arguments, "cluster_alias")
	limit := int(getFloatArg(arguments, "limit", 0)) // 0 表示不限

	// 强制过滤保护：必须提供至少一个过滤条件，防止拉取全量数据导致超时/OOM
	if ruleGroup == "" && file == "" && group == "" && bi == "" && cluster == "" && kubeClusterAlias == "" && clusterAlias == "" {
		return errorResult("get_rules 拒绝全量拉取：请至少提供一个过滤参数（rule_group / file / group / bi / cluster / kube_cluster_alias / cluster_alias），避免拉取全量 Rules 导致超时或内存溢出。如需概览，请使用 limit 参数限制返回数量。"), nil
	}

	// 构建服务端过滤参数
	var ruleGroups []string
	var files []string
	if ruleGroup != "" {
		ruleGroups = append(ruleGroups, ruleGroup)
	}
	if file != "" {
		files = append(files, file)
	}

	result, err := h.Server.ThanosClient.Rules(ctx, ruleType, ruleGroups, files)
	if err != nil {
		return errorResult(fmt.Sprintf("Error fetching rules: %v", err)), nil
	}
	if result.Status != "success" {
		return errorResult(fmt.Sprintf("Rules query failed: %s", result.Error)), nil
	}

	if result.Data == nil {
		return successResult(map[string]interface{}{"groups": []interface{}{}}), nil
	}

	output := map[string]interface{}{}

	// 过滤逻辑
	var filteredGroups []interface{}
	ruleCount := 0

	for _, g := range result.Data.Groups {
		groupMap, ok := g.(map[string]interface{})
		if !ok {
			continue
		}

		// group 名称过滤（AND 关系）
		if group != "" {
			name, _ := groupMap["name"].(string)
			if !strings.Contains(strings.ToLower(name), strings.ToLower(group)) {
				continue
			}
		}

		// 过滤组内的 rules
		rules, _ := groupMap["rules"].([]interface{})
		var filteredRules []interface{}
		for _, r := range rules {
			ruleMap, ok := r.(map[string]interface{})
			if !ok {
				continue
			}

			// bi/cluster/kube_cluster_alias/cluster_alias 过滤（OR 关系，匹配任意一个即可）
			if bi != "" || cluster != "" || kubeClusterAlias != "" || clusterAlias != "" {
				labels, _ := ruleMap["labels"].(map[string]interface{})
				b, _ := labels["bi"].(string)
				c, _ := labels["cluster"].(string)
				kca, _ := labels["kube_cluster_alias"].(string)
				ca, _ := labels["cluster_alias"].(string)

				matched := false
				if bi != "" && b == bi {
					matched = true
				}
				if cluster != "" && c == cluster {
					matched = true
				}
				if kubeClusterAlias != "" && kca == kubeClusterAlias {
					matched = true
				}
				if clusterAlias != "" && ca == clusterAlias {
					matched = true
				}

				if !matched {
					continue
				}
			}

			filteredRules = append(filteredRules, r)
			ruleCount++

			// limit 截断（按规则数限制）
			if limit > 0 && ruleCount >= limit {
				break
			}
		}

		// 如果该组有匹配的规则，加入结果
		if len(filteredRules) > 0 {
			// 创建新的 group 对象，只包含过滤后的 rules
			newGroup := make(map[string]interface{})
			for k, v := range groupMap {
				newGroup[k] = v
			}
			newGroup["rules"] = filteredRules
			filteredGroups = append(filteredGroups, newGroup)

			if limit > 0 && ruleCount >= limit {
				break
			}
		}
	}

	output["groups"] = filteredGroups
	output["group_count"] = len(filteredGroups)

	// 统计规则数量
	totalRules := 0
	alertRules := 0
	recordRules := 0
	for _, g := range filteredGroups {
		if m, ok := g.(map[string]interface{}); ok {
			if rules, ok := m["rules"].([]interface{}); ok {
				totalRules += len(rules)
				for _, r := range rules {
					if rm, ok := r.(map[string]interface{}); ok {
						if t, _ := rm["type"].(string); t == "alerting" {
							alertRules++
						} else {
							recordRules++
						}
					}
				}
			}
		}
	}
	output["total_rules"] = totalRules
	output["alert_rules"] = alertRules
	output["record_rules"] = recordRules

	// 记录过滤条件
	filters := map[string]string{}
	if ruleGroup != "" {
		filters["rule_group"] = ruleGroup
	}
	if file != "" {
		filters["file"] = file
	}
	if group != "" {
		filters["group"] = group
	}
	if bi != "" {
		filters["bi"] = bi
	}
	if cluster != "" {
		filters["cluster"] = cluster
	}
	if kubeClusterAlias != "" {
		filters["kube_cluster_alias"] = kubeClusterAlias
	}
	if clusterAlias != "" {
		filters["cluster_alias"] = clusterAlias
	}
	if len(filters) > 0 {
		output["filters"] = filters
	}
	if limit > 0 {
		output["limit"] = limit
	}

	return successResult(output), nil
}

// handleGetAlerts 获取当前活跃告警（从 /api/v1/rules?type=alert 提取 firing/pending）
func (h *Handler) handleGetAlerts(ctx context.Context, arguments map[string]interface{}) (map[string]interface{}, error) {
	severity := getStringArg(arguments, "severity")

	// 改用 Rules API，type=alert 只返回告警规则
	result, err := h.Server.ThanosClient.Rules(ctx, "alert", []string{}, []string{})
	if err != nil {
		return errorResult(fmt.Sprintf("Error fetching alerts from rules: %v", err)), nil
	}
	if result.Status != "success" {
		return errorResult(fmt.Sprintf("Rules query failed: %s", result.Error)), nil
	}

	if result.Data == nil || len(result.Data.Groups) == 0 {
		return successResult(map[string]interface{}{"alerts": []interface{}{}, "total": 0, "state_summary": map[string]int{"firing": 0, "pending": 0}}), nil
	}

	// 从 rules 中提取 firing/pending 的告警
	var activeAlerts []map[string]interface{}
	for _, g := range result.Data.Groups {
		groupMap, ok := g.(map[string]interface{})
		if !ok {
			continue
		}
		groupName, _ := groupMap["name"].(string)
		groupFile, _ := groupMap["file"].(string)
		rules, _ := groupMap["rules"].([]interface{})

		for _, r := range rules {
			ruleMap, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			// 只处理 alerting 类型（Rules API 已过滤，但再确认一下）
			ruleType, _ := ruleMap["type"].(string)
			if ruleType != "alerting" {
				continue
			}

			state, _ := ruleMap["state"].(string)
			// 只收集 firing 和 pending 状态
			if state != "firing" && state != "pending" {
				continue
			}

			// 按 severity 过滤
			if severity != "" {
				if labels, ok := ruleMap["labels"].(map[string]interface{}); ok {
					if s, _ := labels["severity"].(string); !strings.EqualFold(s, severity) {
						continue
					}
				}
			}

			// 构造告警对象，保留关键字段
			alert := map[string]interface{}{
				"name":      ruleMap["name"],
				"state":     state,
				"labels":    ruleMap["labels"],
				"annotations": ruleMap["annotations"],
				"value":     ruleMap["value"],
				"duration":  ruleMap["duration"],
				"group":     groupName,
				"file":      groupFile,
			}
			activeAlerts = append(activeAlerts, alert)
		}
	}

	// 按状态统计
	stateStats := map[string]int{"firing": 0, "pending": 0}
	for _, a := range activeAlerts {
		if state, ok := a["state"].(string); ok {
			stateStats[state]++
		}
	}

	output := map[string]interface{}{
		"alerts":        activeAlerts,
		"total":         len(activeAlerts),
		"state_summary": stateStats,
		"source":        "rules_api",
	}
	if severity != "" {
		output["filter_severity"] = severity
	}

	return successResult(output), nil
}
