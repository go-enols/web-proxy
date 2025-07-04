package main

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

// CheckAuth 验证客户端认证信息
func CheckAuth(r *http.Request) bool {
	// 如果没有设置认证信息，则不需要验证
	if GlobalConfig.AuthUser == "" || GlobalConfig.AuthPass == "" {
		return true
	}

	// 获取Proxy-Authorization头
	auth := r.Header.Get("Proxy-Authorization")
	if auth == "" {
		return false
	}

	// 检查是否是Basic认证
	if !strings.HasPrefix(auth, "Basic ") {
		return false
	}

	// 解码认证信息
	encoded := auth[6:] // 去掉"Basic "前缀
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return false
	}

	// 分割用户名和密码
	creds := strings.SplitN(string(decoded), ":", 2)
	if len(creds) != 2 {
		return false
	}
	fmt.Println("验证用户名和密码", creds[0] == GlobalConfig.AuthUser && creds[1] == GlobalConfig.AuthPass)

	// 验证用户名和密码
	return creds[0] == GlobalConfig.AuthUser && creds[1] == GlobalConfig.AuthPass
}

// SendAuthRequired 发送407认证要求响应
func SendAuthRequired(w http.ResponseWriter) {
	w.Header().Set("Proxy-Authenticate", "Basic realm=\"Proxy\"")
	w.WriteHeader(http.StatusProxyAuthRequired)
	w.Write([]byte("Proxy Authentication Required"))
}
