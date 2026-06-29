package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"thanos-mcp/config"
	"thanos-mcp/tools"
)

// StreamableHTTPHandler 实现 MCP Streamable HTTP 规范
type StreamableHTTPHandler struct {
	server  *tools.MCPServer
	handler *tools.Handler
	logger  Logger
}

// NewStreamableHTTPHandler 创建 Streamable HTTP 处理器
func NewStreamableHTTPHandler(server *tools.MCPServer, logger Logger) *StreamableHTTPHandler {
	return &StreamableHTTPHandler{
		server:  server,
		handler: &tools.Handler{Server: server},
		logger:  logger,
	}
}

func (h *StreamableHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	clientIP := GetClientIP(r)
	authHeader := r.Header.Get("Authorization")
	username := h.server.Auth.ExtractUsername(authHeader)

	// 调试：记录完整请求
	if h.logger != nil {
		h.logger.Debugf("DEBUG 请求 | IP: %s | Method: %s | Path: %s | Auth: %s",
			clientIP, r.Method, r.URL.Path, authHeader)
	}

	// 认证下沉到 processRequest 层按 JSON-RPC method 判断，
	// 导入 tools 无需认证，仅 chat 交互需要认证

	// 处理 OAuth 发现端点 (Cherry Studio 需要) - 保持独立路径
	switch r.URL.Path {
	case "/.well-known/openid-configuration", "/.well-known/oauth-authorization-server":
		h.handleOAuthDiscovery(w, r, clientIP)
		return
	case "/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp":
		h.handleOAuthResource(w, r, clientIP, username)
		return
	case "/register":
		h.handleOAuthRegister(w, r, clientIP)
		return
	}

	// MCP 路由 (使用 /mcp 作为根路径，符合 MCP Streamable HTTP 规范)
	switch {
	case r.URL.Path == "/mcp" || r.URL.Path == "/mcp/":
		switch r.Method {
		case http.MethodGet:
			h.handleSSE(w, r, clientIP, username)
		case http.MethodPost:
			h.handleJSONRPC(w, r, clientIP, username, authHeader)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	default:
		h.logger.Accessf("请求 404 | IP: %-15s | User: %-15s | Method: %-8s | Path: %s",
			clientIP, username, r.Method, r.URL.Path)
		http.Error(w, "Not Found: MCP endpoint is /mcp", http.StatusNotFound)
	}
}

// handleOAuthDiscovery 处理 OAuth 发现端点
func (h *StreamableHTTPHandler) handleOAuthDiscovery(w http.ResponseWriter, r *http.Request, clientIP string) {
	h.logger.Infof("OAuth 发现请求 | IP: %s | Path: %s", clientIP, r.URL.Path)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"issuer":                                "thanos-mcp",
		"authorization_endpoint":                "http://localhost:8080/authorize",
		"token_endpoint":                        "http://localhost:8080/token",
		"resource_endpoint":                     "http://localhost:8080/",
		"jwks_uri":                              "http://localhost:8080/jwks",
		"scopes_supported":                      []string{"read", "write"},
		"bearer_methods_supported":              []string{"header", "query"},
		"resource_signing_alg_values_supported": []string{"RS256", "HS256"},
	}
	json.NewEncoder(w).Encode(response)
}

// handleOAuthResource 处理 OAuth 资源端点
func (h *StreamableHTTPHandler) handleOAuthResource(w http.ResponseWriter, r *http.Request, clientIP, _ string) {
	h.logger.Infof("OAuth 资源请求 | IP: %s | Path: %s", clientIP, r.URL.Path)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("WWW-Authenticate", `Bearer realm="MCP Server"`)

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Unauthorized: Bearer token required", http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"resource": "thanos-mcp",
		"status":   "active",
	})
}

// handleOAuthRegister 处理 OAuth 动态注册
func (h *StreamableHTTPHandler) handleOAuthRegister(w http.ResponseWriter, r *http.Request, clientIP string) {
	// 解析注册请求体
	var registrationReq map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&registrationReq); err != nil {
		h.logger.Errorf("OAuth 注册请求解析失败 | IP: %s | Error: %v", clientIP, err)
	}

	h.logger.Infof("OAuth 注册请求 | IP: %s | ClientID: %v", clientIP, registrationReq["client_id"])

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusCreated)

	response := map[string]interface{}{
		"client_id":                  "thanos-mcp-client",
		"client_name":                "thanos-mcp",
		"redirect_uris":              []string{"http://localhost:8080/callback"},
		"grant_types":                []string{"client_credentials", "authorization_code"},
		"response_types":             []string{"code", "token"},
		"token_endpoint_auth_method": "client_secret_basic",
	}
	json.NewEncoder(w).Encode(response)
}

func (h *StreamableHTTPHandler) handleSSE(w http.ResponseWriter, r *http.Request, clientIP, username string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

	h.logger.Infof("SSE 连接已建立 | IP: %s | User: %s", clientIP, username)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	clientGone := r.Context().Done()
	for {
		select {
		case <-clientGone:
			h.logger.Infof("SSE 连接关闭 | IP: %s | User: %s", clientIP, username)
			return
		case <-ticker.C:
			fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
			flusher.Flush()
		}
	}
}

func (h *StreamableHTTPHandler) handleJSONRPC(w http.ResponseWriter, r *http.Request, clientIP, username string, authHeader string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Errorf("JSON解析失败 | IP: %s | Error: %v", clientIP, err)
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &JSONError{
				Code:    -32700,
				Message: "Parse error: " + err.Error(),
			},
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := h.processRequest(&req, clientIP, username, authHeader)

	// 必须设置 ID（JSON-RPC 规范要求响应 ID 与请求 ID 一致）
	resp.ID = req.RequestID()

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Errorf("响应编码失败 | IP: %s | Error: %v", clientIP, err)
	}
}

func (h *StreamableHTTPHandler) processRequest(req *JSONRPCRequest, clientIP, username string, authHeader string) JSONRPCResponse {
	// 导入 tools 相关的方法无需认证，仅 chat 交互需要认证
	switch req.Method {
	case "initialize", "ping", "tools/list", "prompts/list", "resources/list":
	default:
		if h.server.Auth.Enabled {
			// 优先使用 HTTP 层传递的 authHeader，其次从请求参数中提取
			checkHeader := authHeader
			if checkHeader == "" {
				if header, ok := req.Params["_auth_header"].(string); ok {
					checkHeader = header
				} else if meta, ok := req.Params["_meta"].(map[string]interface{}); ok {
					if auth, ok := meta["authorization"].(string); ok {
						checkHeader = auth
					}
				} else if extra, ok := req.Params["_extraHeaders"].(map[string]interface{}); ok {
					if auth, ok := extra["Authorization"].(string); ok {
						checkHeader = auth
					}
				}
			}
			if !h.server.Auth.ValidateAuth(checkHeader) {
				h.logger.Warnf("认证失败 | IP: %s | Method: %s", clientIP, req.Method)
				return JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.RequestID(),
					Error: &JSONError{
						Code:    -32603,
						Message: "Unauthorized",
					},
				}
			}
		}
	}

	// 认证通过后记录请求
	switch req.Method {
	case "tools/list":
		h.logger.Infof("tools/list | IP: %s | User: %s", clientIP, username)
	case "tools/call":
		name, _ := req.Params["name"].(string)
		args, _ := req.Params["arguments"].(map[string]interface{})
		userInfo := username
		if username == "unknown" {
			userInfo = fmt.Sprintf("unknown (token: %s)", config.MaskBearerToken(config.ExtractBearerToken(authHeader)))
		}
		h.logger.Accessf("MCP REQUEST | IP: %-15s | User: %-50s | Tool: %s | Args: %v",
			clientIP, userInfo, name, args)
	case "mcp:restart-server":
		h.logger.Infof("mcp:restart-server | IP: %s | User: %s", clientIP, username)
	case "mcp:shutdown":
		h.logger.Infof("mcp:shutdown | IP: %s | User: %s", clientIP, username)
	}

	return ProcessToolsRequest(h.handler, req)
}

// RunStreamableHttpTransport 启动 Streamable HTTP 传输模式
func RunStreamableHttpTransport(server *tools.MCPServer, port int, logger Logger) {
	logger.Infof("========================================")
	logger.Infof("Thanos MCP Server 启动 (streamable-http mode)")
	logger.Infof("Thanos endpoint: %s", server.ThanosClient.Endpoint)
	if server.Auth.Enabled {
		logger.Infof("认证: 已启用 | 用户数: %d", len(server.Auth.Users))
	} else {
		logger.Infof("认证: 已禁用")
	}
	logger.Infof("========================================")

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Streamable HTTP Server listening on %s\n", addr)
	handler := NewStreamableHTTPHandler(server, logger)
	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.Errorf("服务器错误: %v", err)
		fmt.Printf("Server error: %v\n", err)
	}
}
