package main

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
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
func HandleProxyTunneling(w http.ResponseWriter, r *http.Request) {
	// 验证认证信息
	if !CheckAuth(r) {
		SendAuthRequired(w)
		return
	}
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

	// 如果需要认证，设置代理服务器的认证信息
	authorizationHeader := ""
	if auth != "" {
		encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
		authorizationHeader = "Proxy-Authorization: Basic " + encodedAuth + "\r\n"
	}

	// 发送CONNECT请求给第二级代理
	connectRequest := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n%s\r\n", r.Host, r.Host, authorizationHeader)
	proxyConn.Write([]byte(connectRequest))
	resp, err := http.ReadResponse(bufio.NewReader(proxyConn), r)
	if err != nil {
		http.Error(w, "Failed to read response from the second proxy", http.StatusServiceUnavailable)
		return
	}
	if resp.StatusCode != 200 {
		http.Error(w, "Failed to connect to the host through the second proxy", resp.StatusCode)
		return
	}

	// 响应客户端的CONNECT请求，发送200 OK状态码
	w.WriteHeader(http.StatusOK)

	// 劫持连接
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

	// 从此点开始，不要再使用w来写入响应

	// 开始转发数据
	go Transfer(clientConn, proxyConn)
	go Transfer(proxyConn, clientConn)
}

// HandleDirectTunneling 处理直接转发的HTTPS隧道请求
func HandleDirectTunneling(w http.ResponseWriter, r *http.Request) {
	// 验证认证信息
	if !CheckAuth(r) {
		SendAuthRequired(w)
		return
	}
	// 直接连接目标服务器
	destConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, "Failed to connect to the host", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
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

	// 开始转发数据
	go Transfer(destConn, clientConn)
	go Transfer(clientConn, destConn)
}

// HandleProxyHTTP 处理通过第二级代理转发的HTTP请求
func HandleProxyHTTP(w http.ResponseWriter, r *http.Request) {
	if !CheckAuth(r) {
		SendAuthRequired(w)
		return
	}
	// 使用配置了第二级代理的http.Transport发送请求
	proxy := httputil.NewSingleHostReverseProxy(nil)
	proxy.Transport = proxyTransport
	proxy.ServeHTTP(w, r)
}

// HandleDirectHTTP 处理直接转发的HTTP请求
func HandleDirectHTTP(w http.ResponseWriter, r *http.Request) {
	// 验证认证信息
	if !CheckAuth(r) {
		SendAuthRequired(w)
		return
	}
	// 使用默认的http.Transport发送请求
	proxy := httputil.NewSingleHostReverseProxy(nil)
	proxy.ServeHTTP(w, r)
}