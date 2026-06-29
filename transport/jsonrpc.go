package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"thanos-mcp/prompts"
	"thanos-mcp/tools"
)

// DefaultContext 默认上下文
var DefaultContext = context.Background()

// JSONRPCRequest JSON-RPC 请求
type JSONRPCRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      json.RawMessage        `json:"id,omitempty"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// JSONRPCResponse JSON-RPC 响应
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *JSONError  `json:"error,omitempty"`
}

// MarshalJSON 自定义序列化，确保不输出 null 字段
func (r JSONRPCResponse) MarshalJSON() ([]byte, error) {
	type alias JSONRPCResponse

	// 构建最小化的响应，只包含必要字段
	m := map[string]interface{}{
		"jsonrpc": r.JSONRPC,
	}

	// ID: 只有非 nil 且非空时才输出
	if r.ID != nil {
		m["id"] = r.ID
	}

	// result 和 error 互斥：只输出有值的那个
	if r.Error != nil {
		m["error"] = r.Error
	} else if r.Result != nil {
		m["result"] = r.Result
	}

	return json.Marshal(m)
}

// JSONError JSON-RPC 错误
type JSONError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// IsNotification 检查是否为通知请求（无 id）
func (r *JSONRPCRequest) IsNotification() bool {
	return len(r.ID) == 0
}

// RequestID 返回请求 ID（用于响应）
func (r *JSONRPCRequest) RequestID() interface{} {
	if len(r.ID) == 0 {
		return nil
	}
	var id interface{}
	if err := json.Unmarshal(r.ID, &id); err != nil {
		return nil
	}
	return id
}

// ProcessToolsRequest 处理工具相关请求
func ProcessToolsRequest(handler *tools.Handler, req *JSONRPCRequest) JSONRPCResponse {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
	}

	switch req.Method {
	case "initialize":
		pv := "2024-11-05"
		if req.Params != nil {
			if v, ok := req.Params["protocolVersion"].(string); ok && v != "" {
				pv = v
			}
		}
		response.Result = map[string]interface{}{
			"protocolVersion": pv,
			"capabilities": map[string]interface{}{
				"tools": map[string]bool{
					"list": true,
					"call": true,
				},
				"prompts": map[string]bool{
					"list": true,
				},
				"resources": map[string]bool{
					"list":      true,
					"subscribe": false,
				},
				"sampling": map[string]bool{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "thanos-mcp",
				"version": "1.0.0",
			},
		}

	case "ping":
		response.Result = map[string]interface{}{}

	case "tools/list":
		response.Result = map[string]interface{}{
			"tools": tools.GetToolDefinitions(),
		}

	case "tools/call":
		params := req.Params
		if params == nil {
			params = map[string]any{}
		}
		name, _ := params["name"].(string)
		arguments, _ := params["arguments"].(map[string]interface{})

		result, err := handler.HandleCallTool(DefaultContext, name, arguments)
		if err != nil {
			response.Error = &JSONError{
				Code:    -32603,
				Message: err.Error(),
			}
		} else {
			response.Result = result
		}

	case "mcp:restart-server":
		response.Result = map[string]interface{}{
			"success": true,
			"message": "Server restart requested. Please manually restart.",
		}

	case "mcp:shutdown":
		response.Result = map[string]interface{}{
			"success": true,
			"message": "Server shutdown requested",
		}

	case "mcp:server-info":
		response.Result = map[string]interface{}{
			"name":    "thanos-mcp",
			"version": "1.0.0",
		}

	case "sampling/createMessage":
		response.Result = map[string]interface{}{
			"model":      "thanos-mcp",
			"stopReason": "end_turn",
			"content": []map[string]string{
				{"type": "text", "text": "This server does not support AI sampling."},
			},
		}

	case "prompts/list":
		response.Result = map[string]interface{}{
			"prompts": prompts.GetAllPrompts(),
		}

	case "prompts/get":
		var params struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments"`
		}
		if req.Params != nil {
			raw, _ := json.Marshal(req.Params)
			json.Unmarshal(raw, &params)
		}
		result := prompts.HandleGetPrompt(params.Name, params.Arguments)
		response.Result = result

	case "resources/list":
		response.Result = map[string]interface{}{
			"resources": []interface{}{},
		}

	case "roots/list":
		response.Result = map[string]interface{}{
			"roots": []interface{}{},
		}

	case "roots/add":
		response.Result = map[string]interface{}{
			"success": true,
		}

	default:
		response.Error = &JSONError{
			Code:    -32601,
			Message: fmt.Sprintf("Method not found: %s", req.Method),
		}
	}

	return response
}

// GetClientIP 获取客户端IP（优先 IPv4）
// 支持跨集群 k8s 场景，按优先级依次检查代理头
func GetClientIP(r *http.Request) string {
	// 1. X-Forwarded-For: 最常见的代理头，取第一个（最左侧为原始客户端 IP）
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip := strings.Split(xff, ",")[0]
		return normalizeIP(strings.TrimSpace(ip))
	}

	// 2. X-Real-IP: Nginx 等反向代理常用
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return normalizeIP(strings.TrimSpace(xri))
	}

	// 3. Forwarded: RFC 7239 标准头，格式如 "for=192.168.1.1;proto=http"
	if fwd := r.Header.Get("Forwarded"); fwd != "" {
		if ip := parseForwardedHeader(fwd); ip != "" {
			return normalizeIP(ip)
		}
	}

	// 4. X-Original-Forwarded-For: 某些 Ingress Controller（如 Nginx Ingress）使用
	if xoff := r.Header.Get("X-Original-Forwarded-For"); xoff != "" {
		ip := strings.Split(xoff, ",")[0]
		return normalizeIP(strings.TrimSpace(ip))
	}

	// 5. X-Envoy-External-Address: Istio / Envoy 场景
	if envoy := r.Header.Get("X-Envoy-External-Address"); envoy != "" {
		return normalizeIP(strings.TrimSpace(envoy))
	}

	// 6. 最后使用 RemoteAddr
	addr := r.RemoteAddr
	if host, _, err := net.SplitHostPort(addr); err == nil {
		addr = host
	}

	return normalizeIP(addr)
}

// parseForwardedHeader 解析 RFC 7239 Forwarded 头，提取第一个 for= 的 IP
func parseForwardedHeader(fwd string) string {
	// 取第一段（可能有多个代理，逗号分隔）
	first := strings.Split(fwd, ",")[0]
	for _, part := range strings.Split(first, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "for=") {
			ip := part[4:]
			// 去除引号和方括号（IPv6 格式 "for=[::1]:port"）
			ip = strings.Trim(ip, `"`)
			ip = strings.Trim(ip, `[]`)
			// 如果包含端口，去掉端口
			if host, _, err := net.SplitHostPort(ip); err == nil {
				ip = host
			}
			return ip
		}
	}
	return ""
}

// normalizeIP 标准化 IP 地址，将 IPv6 环回和映射地址转为 IPv4
func normalizeIP(addr string) string {
	if addr == "::1" {
		return "127.0.0.1"
	}
	if strings.Contains(addr, ":") && !strings.HasPrefix(addr, "[") {
		if ip := toIPv4(addr); ip != "" {
			return ip
		}
	}
	return addr
}

// toIPv4 将 IPv6 映射的 IPv4 地址转换（如 ::ffff:127.0.0.1 -> 127.0.0.1）
func toIPv4(ipv6 string) string {
	if strings.HasPrefix(ipv6, "::ffff:") {
		return strings.TrimPrefix(ipv6, "::ffff:")
	}
	return ""
}
