package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zuoyangs/go-mcp-common/serverauth"
	"thanos-mcp/thanos"
)

// Handler 工具处理器
type Handler struct {
	Server *MCPServer
}

// MCPServer MCP 服务器
type MCPServer struct {
	ThanosClient *thanos.Client
	Transport    string
	Auth         serverauth.Config
}

// NewMCPServer creates a new MCP server instance
func NewMCPServer(thanosEndpoint string, thanosTimeout time.Duration, transport string, auth serverauth.Config) *MCPServer {
	thanosClient := thanos.NewClient(thanosEndpoint, thanosTimeout)

	return &MCPServer{
		ThanosClient: thanosClient,
		Transport:    transport,
		Auth:         auth,
	}
}

// UpdateThanosEndpoint 更新 Thanos endpoint（用于动态更新配置）
func (s *MCPServer) UpdateThanosEndpoint(endpoint string) {
	s.ThanosClient.Endpoint = endpoint
}

// GetThanosEndpoint 获取当前配置的 endpoint
func (s *MCPServer) GetThanosEndpoint() string {
	return s.ThanosClient.Endpoint
}

// logEndpoint 输出当前 endpoint 配置的调试日志
func (s *MCPServer) logEndpoint() {
	fmt.Printf("[DEBUG] Thanos Endpoint 配置: %s\n", s.ThanosClient.Endpoint)
}

// HandleCallTool processes tool calls
func (h *Handler) HandleCallTool(ctx context.Context, name string, arguments map[string]interface{}) (interface{}, error) {
	switch name {
	case "query":
		return h.handleQuery(ctx, arguments)
	case "query_range":
		return h.handleQueryRange(ctx, arguments)
	case "get-targets":
		return h.handleGetTargets(ctx, arguments)
	case "get-rules":
		return h.handleGetRules(ctx, arguments)
	case "get-alerts":
		return h.handleGetAlerts(ctx, arguments)
	case "get-cluster-info":
		return h.handleGetClusterInfo(ctx, arguments)
	case "get-status":
		return h.handleGetStatus(ctx, arguments)
	case "get-stores":
		return h.handleGetStores(ctx, arguments)
	case "get-cardinality-analysis":
		return h.handleCardinalityAnalysis(ctx, arguments)
	default:
		return errorResult(fmt.Sprintf("Unknown tool: %s", name)), nil
	}
}

// ─── 辅助函数 ───

const dataIntegrityReminder = "\n\n[IMPORTANT] 以上数据均来自实际工具查询结果。请严格基于上述返回数据进行分析和回答，严禁编造、捏造或虚构任何未在上述结果中出现的数据。如果数据不足以回答用户问题，请如实告知。"

func successResult(data interface{}) map[string]interface{} {
	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return errorResult(fmt.Sprintf("Error formatting result: %v", err))
	}
	return map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": string(output) + dataIntegrityReminder},
		},
	}
}

func errorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": msg + "\n\n[IMPORTANT] 工具调用失败，请如实告知用户查询失败，严禁编造数据。"},
		},
		"isError": true,
	}
}

func getStringArg(args map[string]interface{}, key string) string {
	v, _ := args[key].(string)
	return v
}

func getFloatArg(args map[string]interface{}, key string, def float64) float64 {
	v, ok := args[key].(float64)
	if !ok {
		return def
	}
	return v
}
