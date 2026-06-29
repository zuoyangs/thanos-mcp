package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"

	"github.com/zuoyangs/go-mcp-common/serverauth"
)

// TransportMode 传输模式常量
const (
	TransportStdio          = "stdio"
	TransportStreamableHttp = "streamable-http"
	TransportHTTP           = "http"
)

// ServerConfig 服务器配置
type ServerConfig struct {
	ThanosEndpoint string
	ThanosTimeout  time.Duration
	Transport      string
	Port           int
	Auth           serverauth.Config
}

// LoadConfig 加载完整配置，configFile 为配置文件路径
func LoadConfig(configFile string) (*ServerConfig, error) {
	v := viper.New()
	v.SetConfigType("yaml")

	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		v.AddConfigPath("etc")
		v.AddConfigPath("./etc")
		v.AddConfigPath("../etc")
		v.SetConfigName("config")
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	fmt.Printf("使用配置文件: %s\n", v.ConfigFileUsed())

	return &ServerConfig{
		ThanosEndpoint: getThanosEndpoint(v),
		ThanosTimeout:  getThanosTimeout(v),
		Transport:      getTransport(v),
		Port:           getServerPort(v),
		Auth:           getAuthConfig(v),
	}, nil
}

func getThanosEndpoint(v *viper.Viper) string {
	// 环境变量优先级最高
	if endpoint := os.Getenv("THANOS_ENDPOINT"); endpoint != "" {
		return endpoint
	}
	if endpoint := os.Getenv("MCP_THANOS_ENDPOINT"); endpoint != "" {
		return endpoint
	}
	return v.GetString("thanos.endpoint")
}

func getThanosTimeout(v *viper.Viper) time.Duration {
	timeout := 30 * time.Second
	if t := v.GetString("thanos.timeout"); t != "" {
		if dur, err := time.ParseDuration(t); err == nil {
			timeout = dur
		}
	}
	return timeout
}

func getTransport(v *viper.Viper) string {
	transport := os.Getenv("MCP_TRANSPORT")
	if transport == "" {
		transport = v.GetString("mcp.transport")
		if transport == "" {
			transport = TransportStdio
		}
	}
	return transport
}

func getServerPort(v *viper.Viper) int {
	port := os.Getenv("MCP_PORT")
	serverPort := v.GetInt("mcp.port")
	if serverPort == 0 {
		serverPort = 8080
	}
	if port != "" {
		fmt.Sscanf(port, "%d", &serverPort)
	}
	return serverPort
}

func getAuthConfig(v *viper.Viper) serverauth.Config {
	var users []serverauth.User

	// Try to load users from config
	type viperUser struct {
		Username string `mapstructure:"username"`
		Password string `mapstructure:"password"`
		Token    string `mapstructure:"token"`
	}
	var vUsers []viperUser
	if err := v.UnmarshalKey("auth.users", &vUsers); err == nil {
		for _, u := range vUsers {
			users = append(users, serverauth.User{
				Username: u.Username,
				Password: u.Password,
				Token:    u.Token,
			})
		}
	}

	if len(users) == 0 {
		username := v.GetString("auth.username")
		password := v.GetString("auth.password")
		if username != "" && password != "" {
			users = append(users, serverauth.User{Username: username, Password: password})
		}
	}

	// 读取全局 Bearer token
	token := v.GetString("auth.token")
	if token == "" {
		token = os.Getenv("MCP_AUTH_TOKEN")
	}

	authEnabledEnv := os.Getenv("MCP_AUTH_ENABLED")
	enabled := v.GetBool("auth.enabled")
	if authEnabledEnv != "" {
		enabled = authEnabledEnv == "true" || authEnabledEnv == "1"
	}

	hasUsers := len(users) > 0
	hasToken := token != ""
	authEnabled := (hasUsers || hasToken) && enabled

	return serverauth.Config{
		Users:   users,
		Enabled: authEnabled,
		Token:   token,
	}
}

// ExtractBearerToken re-exports serverauth.ExtractBearerToken for backward compatibility.
var ExtractBearerToken = serverauth.ExtractBearerToken

// MaskBearerToken re-exports serverauth.MaskToken for backward compatibility.
var MaskBearerToken = serverauth.MaskToken
