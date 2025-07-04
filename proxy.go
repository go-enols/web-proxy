package main

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
)

// 创建一个代理配置用于第二级代理的http.Transport
var proxyTransport = &http.Transport{
	Proxy: func(_ *http.Request) (*url.URL, error) {
		proxyStr, _, err := ParseProxyURL(GlobalConfig.ProxyURL)
		if err != nil {
			log.Println("Error parsing proxy URL:", err)
			return nil, err
		}
		return url.Parse(proxyStr)
	},
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // 如果第二级代理使用自签名证书，需要跳过证书验证
}

// HandleProxyTunneling 处理通过第二级代理转发的HTTPS隧道请求
// 注意：这个实现会隐藏客户端真实IP，目标服务器只能看到代理服务器IP
func HandleProxyTunneling(w http.ResponseWriter, r *http.Request) {
	// 验证认证信息
	if !CheckAuth(r) {
		SendAuthRequired(w)
		return
	}
	
	// 解析目标地址
	targetHost := r.Host
	if targetHost == "" {
		http.Error(w, "Missing target host", http.StatusBadRequest)
		return
	}

	// 通过第二级代理连接到目标服务器
	proxyStr, auth, err := ParseProxyURL(GlobalConfig.ProxyURL)
	if err != nil {
		http.Error(w, "Failed to parse proxy URL", http.StatusInternalServerError)
		return
	}

	// 连接到第二级代理服务器
	proxyConn, err := net.Dial("tcp", proxyStr)
	if err != nil {
		http.Error(w, "Failed to connect to the second proxy", http.StatusServiceUnavailable)
		return
	}
	defer proxyConn.Close()

	// 如果需要认证，设置代理服务器的认证信息
	authorizationHeader := ""
	if auth != "" {
		encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
		authorizationHeader = "Proxy-Authorization: Basic " + encodedAuth + "\r\n"
	}

	// 发送CONNECT请求给第二级代理
	connectRequest := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n%s\r\n", targetHost, targetHost, authorizationHeader)
	_, err = proxyConn.Write([]byte(connectRequest))
	if err != nil {
		http.Error(w, "Failed to send CONNECT request", http.StatusServiceUnavailable)
		return
	}

	// 读取代理服务器的响应
	resp, err := http.ReadResponse(bufio.NewReader(proxyConn), r)
	if err != nil {
		http.Error(w, "Failed to read response from the second proxy", http.StatusServiceUnavailable)
		return
	}
	if resp.StatusCode != 200 {
		http.Error(w, fmt.Sprintf("Proxy connection failed: %d %s", resp.StatusCode, resp.Status), resp.StatusCode)
		return
	}

	// 响应客户端的CONNECT请求
	w.WriteHeader(http.StatusOK)

	// 劫持客户端连接
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()

	// 开始双向数据转发
	// 这里的关键是：代理服务器作为中间人，客户端和目标服务器都无法直接看到对方
	go Transfer(clientConn, proxyConn)
	go Transfer(proxyConn, clientConn)

	// 等待连接关闭
	select {}
}

// HandleDirectTunneling 处理直接转发的HTTPS隧道请求
// 注意：这个实现会隐藏客户端真实IP，目标服务器只能看到代理服务器IP
func HandleDirectTunneling(w http.ResponseWriter, r *http.Request) {
	// 验证认证信息
	if !CheckAuth(r) {
		SendAuthRequired(w)
		return
	}
	
	// 解析目标地址
	targetHost := r.Host
	if targetHost == "" {
		http.Error(w, "Missing target host", http.StatusBadRequest)
		return
	}
	
	// 直接连接目标服务器
	destConn, err := net.DialTimeout("tcp", targetHost, 10*time.Second)
	if err != nil {
		http.Error(w, "Failed to connect to the host", http.StatusServiceUnavailable)
		return
	}
	defer destConn.Close()
	
	// 响应客户端的CONNECT请求
	w.WriteHeader(http.StatusOK)
	
	// 劫持客户端连接
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()

	// 开始双向数据转发
	// 代理服务器作为中间人，隐藏客户端真实IP
	go Transfer(destConn, clientConn)
	go Transfer(clientConn, destConn)
	
	// 等待连接关闭
	select {}
}

// HandleProxyHTTP 处理通过第二级代理转发的HTTP请求
// 注意：这个实现会隐藏客户端真实IP，目标服务器只能看到代理服务器IP
func HandleProxyHTTP(w http.ResponseWriter, r *http.Request) {
	if !CheckAuth(r) {
		SendAuthRequired(w)
		return
	}
	
	// 移除可能暴露客户端信息的头部
	r.Header.Del("X-Forwarded-For")
	r.Header.Del("X-Real-IP")
	r.Header.Del("X-Forwarded-Proto")
	r.Header.Del("X-Forwarded-Host")
	
	// 创建新的请求，确保通过代理转发
	client := &http.Client{
		Transport: proxyTransport,
		Timeout:   30 * time.Second,
	}
	
	// 构建完整的目标URL
	targetURL := r.URL
	if !targetURL.IsAbs() {
		// 如果URL不是绝对路径，构建完整URL
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		targetURL = &url.URL{
			Scheme: scheme,
			Host:   r.Host,
			Path:   r.URL.Path,
			RawQuery: r.URL.RawQuery,
		}
	}
	
	// 创建新请求
	proxyReq, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}
	
	// 复制头部（除了已删除的那些）
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}
	
	// 发送请求
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "Failed to proxy request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	
	// 复制响应头部
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	
	// 设置状态码
	w.WriteHeader(resp.StatusCode)
	
	// 复制响应体
	io.Copy(w, resp.Body)
}

// HandleDirectHTTP 处理直接转发的HTTP请求
// 注意：这个实现会隐藏客户端真实IP，目标服务器只能看到代理服务器IP
func HandleDirectHTTP(w http.ResponseWriter, r *http.Request) {
	// 验证认证信息
	if !CheckAuth(r) {
		SendAuthRequired(w)
		return
	}
	
	// 移除可能暴露客户端信息的头部
	r.Header.Del("X-Forwarded-For")
	r.Header.Del("X-Real-IP")
	r.Header.Del("X-Forwarded-Proto")
	r.Header.Del("X-Forwarded-Host")
	
	// 创建HTTP客户端进行直接转发
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	// 构建完整的目标URL
	targetURL := r.URL
	if !targetURL.IsAbs() {
		// 如果URL不是绝对路径，构建完整URL
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		targetURL = &url.URL{
			Scheme: scheme,
			Host:   r.Host,
			Path:   r.URL.Path,
			RawQuery: r.URL.RawQuery,
		}
	}
	
	// 创建新请求
	proxyReq, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}
	
	// 复制头部（除了已删除的那些）
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}
	
	// 发送请求
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "Failed to proxy request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	
	// 复制响应头部
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	
	// 设置状态码
	w.WriteHeader(resp.StatusCode)
	
	// 复制响应体
	io.Copy(w, resp.Body)
}