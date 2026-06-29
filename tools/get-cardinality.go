package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"thanos-mcp/thanos"
)

// handleCardinalityAnalysis 高基数分析
func (h *Handler) handleCardinalityAnalysis(ctx context.Context, arguments map[string]interface{}) (map[string]interface{}, error) {
	prometheusEndpoint := getStringArg(arguments, "prometheus_endpoint")
	cluster := getStringArg(arguments, "cluster")
	bi := getStringArg(arguments, "bi")
	kubeClusterAlias := getStringArg(arguments, "kube_cluster_alias")
	limitF := getFloatArg(arguments, "limit", 20)
	limit := int(limitF)

	// prometheus_endpoint 是必填参数
	if prometheusEndpoint == "" {
		return errorResult("Error: 必须提供 'prometheus_endpoint' 参数，即目标 Prometheus 实例的地址（如 http://prometheus:9090）。Thanos Query 不暴露 /api/v1/status/tsdb，需要直接向 Prometheus 发请求。"), nil
	}

	// 至少需要 bi、cluster 或 kube_cluster_alias 之一（用于 PromQL 补充查询）
	if bi == "" && cluster == "" && kubeClusterAlias == "" {
		return errorResult("Error: 必须提供 'bi'、'cluster' 或 'kube_cluster_alias' 参数之一，用于限定 PromQL 补充查询的范围。例如: bi=\"analytics\" 或 cluster=\"prod\" 或 kube_cluster_alias=\"prod-k8s\""), nil
	}

	h.Server.logEndpoint()

	output := map[string]interface{}{
		"analysis_target": map[string]interface{}{
			"prometheus_endpoint": prometheusEndpoint,
			"thanos_endpoint":     h.Server.ThanosClient.Endpoint,
			"bi":                  bi,
			"cluster":             cluster,
			"kube_cluster_alias":  kubeClusterAlias,
			"limit":               limit,
		},
	}

	// ══════════════════════════════════════════════════════════════
	// 一、当前指标现状
	// ══════════════════════════════════════════════════════════════

	currentStatus := map[string]interface{}{}

	// ── 1.1 总体规模：直接向 Prometheus 请求 TSDB Status ──
	tsdbResult, err := h.Server.ThanosClient.TSDBStatusFromPrometheus(ctx, prometheusEndpoint, limit)
	if err != nil {
		return errorResult(fmt.Sprintf("Error: 调用 Prometheus TSDB Status API 失败（%s）: %v", prometheusEndpoint, err)), nil
	}
	if tsdbResult.Status != "success" {
		return errorResult(fmt.Sprintf("Prometheus TSDB Status 查询失败: %s (type: %s)", tsdbResult.Error, tsdbResult.ErrorType)), nil
	}

	if tsdbResult.Data != nil {
		// 1.1 总体规模
		if tsdbResult.Data.HeadStats != nil {
			currentStatus["1_1_overall_scale"] = map[string]interface{}{
				"description":       "总体规模（来自 Prometheus TSDB Head Stats）",
				"total_series":      tsdbResult.Data.HeadStats.NumSeries,
				"total_label_pairs": tsdbResult.Data.HeadStats.NumLabelPairs,
				"chunk_count":       tsdbResult.Data.HeadStats.ChunkCount,
				"min_time":          tsdbResult.Data.HeadStats.MinTime,
				"max_time":          tsdbResult.Data.HeadStats.MaxTime,
			}
		}

		// 1.2 指标分类分布：按指标名前缀分类统计
		if len(tsdbResult.Data.SeriesCountByMetricName) > 0 {
			categoryMap := map[string]uint64{}
			for _, entry := range tsdbResult.Data.SeriesCountByMetricName {
				prefix := extractMetricPrefix(entry.Name)
				categoryMap[prefix] += entry.Value
			}
			// 转为排序列表
			var categories []map[string]interface{}
			for prefix, count := range categoryMap {
				categories = append(categories, map[string]interface{}{
					"category":     prefix,
					"series_count": count,
				})
			}
			// 按数量降序排序
			sort.Slice(categories, func(i, j int) bool {
				return categories[i]["series_count"].(uint64) > categories[j]["series_count"].(uint64)
			})
			currentStatus["1_2_metric_category_distribution"] = map[string]interface{}{
				"description": "指标分类分布（按指标名前缀聚合）",
				"categories":  categories,
			}
		}

		// 1.3 Series 数量 Top 10（按指标名）
		if len(tsdbResult.Data.SeriesCountByMetricName) > 0 {
			top10 := tsdbResult.Data.SeriesCountByMetricName
			if len(top10) > 10 {
				top10 = top10[:10]
			}
			currentStatus["1_3_top10_series_by_metric"] = map[string]interface{}{
				"description": "Series 数量 Top 10（按指标名）",
				"metrics":     top10,
			}
		}

		// 1.4 高基数 Label 分布
		if len(tsdbResult.Data.LabelValueCountByLabelName) > 0 {
			currentStatus["1_4_high_cardinality_labels"] = map[string]interface{}{
				"description":              "高基数 Label 分布（按 Label 唯一值数量排序）",
				"label_value_counts":       tsdbResult.Data.LabelValueCountByLabelName,
				"memory_by_label":          tsdbResult.Data.MemoryInBytesByLabelName,
			}
		}

		// 1.5 高频 Label-Value 对
		if len(tsdbResult.Data.SeriesCountByLabelValuePair) > 0 {
			currentStatus["1_5_top_label_value_pairs"] = map[string]interface{}{
				"description":       "高频 Label-Value 对（按 Series 数量排序）",
				"label_value_pairs": tsdbResult.Data.SeriesCountByLabelValuePair,
			}
		}
	}

	// 通过 Thanos PromQL 补充查询
	var promqlFilter string
	if bi != "" && cluster != "" && kubeClusterAlias != "" {
		promqlFilter = fmt.Sprintf(`bi="%s",cluster="%s",kube_cluster_alias="%s"`, bi, cluster, kubeClusterAlias)
	} else if bi != "" && cluster != "" {
		promqlFilter = fmt.Sprintf(`bi="%s",cluster="%s"`, bi, cluster)
	} else if bi != "" && kubeClusterAlias != "" {
		promqlFilter = fmt.Sprintf(`bi="%s",kube_cluster_alias="%s"`, bi, kubeClusterAlias)
	} else if cluster != "" && kubeClusterAlias != "" {
		promqlFilter = fmt.Sprintf(`cluster="%s",kube_cluster_alias="%s"`, cluster, kubeClusterAlias)
	} else if bi != "" {
		promqlFilter = fmt.Sprintf(`bi="%s"`, bi)
	} else if cluster != "" {
		promqlFilter = fmt.Sprintf(`cluster="%s"`, cluster)
	} else {
		promqlFilter = fmt.Sprintf(`kube_cluster_alias="%s"`, kubeClusterAlias)
	}

	// 按 job 统计序列数
	jobQuery := fmt.Sprintf(`count by (job) ({%s})`, promqlFilter)
	jobResult, err := h.Server.ThanosClient.Query(ctx, jobQuery)
	if err == nil && jobResult.Status == "success" {
		currentStatus["series_count_by_job"] = map[string]interface{}{
			"description": "按 Job 维度的序列数分布（来自 Thanos PromQL）",
			"data":        jobResult.Data.Result,
		}
	}

	// 总序列数（PromQL 视角）
	totalQuery := fmt.Sprintf(`count({%s})`, promqlFilter)
	totalResult, err := h.Server.ThanosClient.Query(ctx, totalQuery)
	if err == nil && totalResult.Status == "success" {
		currentStatus["total_series_promql"] = map[string]interface{}{
			"description": "总序列数（来自 Thanos PromQL 视角）",
			"data":        totalResult.Data.Result,
		}
	}

	output["section_1_current_status"] = currentStatus

	// ══════════════════════════════════════════════════════════════
	// 二、高基数根本原因分析（从 topN 动态分析，不硬编码特定指标）
	// ══════════════════════════════════════════════════════════════

	rootCauseAnalysis := map[string]interface{}{}

	if tsdbResult.Data != nil {
		// 原因一：Histogram Bucket 数量过多导致 Series 爆炸
		histogramAnalysis := analyzeHistogramBuckets(tsdbResult.Data.SeriesCountByMetricName)
		rootCauseAnalysis["cause_1_histogram_bucket_explosion"] = histogramAnalysis

		// 原因二：高基数 Label 分析（从 topN 动态分析）
		labelAnalysis := analyzeHighCardinalityLabels(
			tsdbResult.Data.LabelValueCountByLabelName,
			tsdbResult.Data.SeriesCountByLabelValuePair,
			tsdbResult.Data.MemoryInBytesByLabelName,
		)
		rootCauseAnalysis["cause_2_high_cardinality_labels"] = labelAnalysis

		// 原因三：高基数指标分析（从 topN 动态分析，不硬编码特定前缀）
		metricAnalysis := analyzeHighCardinalityMetrics(tsdbResult.Data.SeriesCountByMetricName)
		rootCauseAnalysis["cause_3_high_cardinality_metrics"] = metricAnalysis
	}

	output["section_2_root_cause_analysis"] = rootCauseAnalysis

	return successResult(output), nil
}

// extractMetricPrefix 提取指标名的前缀分类
func extractMetricPrefix(metricName string) string {
	// 常见前缀列表
	prefixes := []string{
		"apiserver_", "kubelet_", "container_", "node_", "kube_",
		"etcd_", "coredns_", "envoy_", "istio_", "prometheus_",
		"go_", "process_", "grpc_", "rest_client_", "workqueue_",
		"scheduler_", "controller_", "up", "scrape_",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(metricName, p) {
			return strings.TrimRight(p, "_")
		}
	}
	// 按第一个 _ 分割
	if idx := strings.Index(metricName, "_"); idx > 0 {
		return metricName[:idx]
	}
	return metricName
}

// analyzeHistogramBuckets 分析 Histogram Bucket 导致的 Series 爆炸
func analyzeHistogramBuckets(seriesByMetric []thanos.TopHeapEntry) map[string]interface{} {
	var histogramMetrics []map[string]interface{}
	var totalHistogramSeries uint64

	for _, entry := range seriesByMetric {
		if strings.HasSuffix(entry.Name, "_bucket") {
			histogramMetrics = append(histogramMetrics, map[string]interface{}{
				"metric":       entry.Name,
				"series_count": entry.Value,
				"base_metric":  strings.TrimSuffix(entry.Name, "_bucket"),
			})
			totalHistogramSeries += entry.Value
		}
	}

	// 计算总 series 数
	var totalSeries uint64
	for _, entry := range seriesByMetric {
		totalSeries += entry.Value
	}

	percentage := float64(0)
	if totalSeries > 0 {
		percentage = float64(totalHistogramSeries) / float64(totalSeries) * 100
	}

	return map[string]interface{}{
		"description":                "Histogram Bucket 数量过多导致 Series 爆炸",
		"detail":                     "每个 Histogram 指标会为每个 bucket 边界生成一条独立的 time series。当 bucket 数量多、label 组合多时，series 数量呈乘法爆炸。",
		"histogram_bucket_metrics":   histogramMetrics,
		"total_histogram_series":     totalHistogramSeries,
		"percentage_of_total":        fmt.Sprintf("%.2f%%", percentage),
		"recommendation":             "减少 Histogram bucket 数量（通过 --storage.tsdb.max-bucket-count 或在 scrape config 中使用 metric_relabel_configs 过滤不需要的 bucket），或改用 Summary 类型。",
	}
}

// analyzeHighCardinalityLabels 分析高基数 Label（从 topN 动态分析）
func analyzeHighCardinalityLabels(
	labelValueCounts []thanos.TopHeapEntry,
	seriesByLabelPair []thanos.TopHeapEntry,
	memoryByLabel []thanos.TopHeapEntry,
) map[string]interface{} {
	// 从 topN 中提取高基数 label
	var highCardinalityLabels []map[string]interface{}
	
	for i, entry := range labelValueCounts {
		// 取前 10 个高基数 label
		if i >= 10 {
			break
		}
		
		// 查找对应的内存占用
		var memoryBytes uint64
		for _, mem := range memoryByLabel {
			if mem.Name == entry.Name {
				memoryBytes = mem.Value
				break
			}
		}
		
		// 查找相关的 label-value pair
		var relatedPairs []map[string]interface{}
		for _, pair := range seriesByLabelPair {
			if strings.HasPrefix(pair.Name, entry.Name+"=") {
				relatedPairs = append(relatedPairs, map[string]interface{}{
					"label_value_pair": pair.Name,
					"series_count":     pair.Value,
				})
				if len(relatedPairs) >= 5 {
					break
				}
			}
		}
		
		highCardinalityLabels = append(highCardinalityLabels, map[string]interface{}{
			"label_name":        entry.Name,
			"unique_values":     entry.Value,
			"rank":              i + 1,
			"memory_bytes":      memoryBytes,
			"top_label_values":  relatedPairs,
		})
	}

	return map[string]interface{}{
		"description":              "高基数 Label 分析（从 TSDB Status TopN 动态获取）",
		"detail":                   "Label 的唯一值数量过多会导致 series 总量成倍增长。每个 label 值都会与其他 label 组合产生新的 series。",
		"high_cardinality_labels":  highCardinalityLabels,
		"recommendation":           "审查高基数 label 的值是否合理，是否存在动态生成的值（如 pod 名称、request ID 等）。考虑通过 metric_relabel_configs 过滤或聚合这些 label。",
	}
}

// analyzeHighCardinalityMetrics 分析高基数指标（从 topN 动态分析，不硬编码特定前缀）
func analyzeHighCardinalityMetrics(seriesByMetric []thanos.TopHeapEntry) map[string]interface{} {
	// 计算总 series 数
	var totalSeries uint64
	for _, entry := range seriesByMetric {
		totalSeries += entry.Value
	}

	// 从 topN 中提取高基数指标（取前 20 个）
	var highCardinalityMetrics []map[string]interface{}
	var topNSeries uint64
	
	for i, entry := range seriesByMetric {
		if i >= 20 {
			break
		}
		
		percentage := float64(0)
		if totalSeries > 0 {
			percentage = float64(entry.Value) / float64(totalSeries) * 100
		}
		
		highCardinalityMetrics = append(highCardinalityMetrics, map[string]interface{}{
			"metric":            entry.Name,
			"series_count":      entry.Value,
			"rank":              i + 1,
			"percentage":        fmt.Sprintf("%.2f%%", percentage),
			"category":          extractMetricPrefix(entry.Name),
		})
		topNSeries += entry.Value
	}

	// 按指标前缀分类统计
	categoryMap := map[string]uint64{}
	for _, entry := range seriesByMetric {
		prefix := extractMetricPrefix(entry.Name)
		categoryMap[prefix] += entry.Value
	}

	// 转为排序列表
	var categories []map[string]interface{}
	for prefix, count := range categoryMap {
		percentage := float64(0)
		if totalSeries > 0 {
			percentage = float64(count) / float64(totalSeries) * 100
		}
		categories = append(categories, map[string]interface{}{
			"category":     prefix,
			"series_count": count,
			"percentage":   fmt.Sprintf("%.2f%%", percentage),
		})
	}
	sort.Slice(categories, func(i, j int) bool {
		return categories[i]["series_count"].(uint64) > categories[j]["series_count"].(uint64)
	})

	// 取前 10 个分类
	if len(categories) > 10 {
		categories = categories[:10]
	}

	topNPercentage := float64(0)
	if totalSeries > 0 {
		topNPercentage = float64(topNSeries) / float64(totalSeries) * 100
	}

	return map[string]interface{}{
		"description":              "高基数指标分析（从 TSDB Status TopN 动态获取）",
		"detail":                   "从 /tsdb-status 的 SeriesCountByMetricName TopN 中分析高基数指标，不预设特定指标前缀。",
		"top20_metrics":            highCardinalityMetrics,
		"top20_series_count":       topNSeries,
		"top20_percentage":         fmt.Sprintf("%.2f%%", topNPercentage),
		"top10_categories":         categories,
		"total_series":             totalSeries,
		"recommendation":           "根据 TopN 指标分析结果，识别高基数指标来源。通过 metric_relabel_configs 过滤不需要的指标或减少 label 维度。重点关注 histogram bucket、高维 label 组合等场景。",
	}
}
