package main

import (
	"fmt"
	"log"
	"net/http"
)

// StartProxyServer 启动二次代理服务器
func StartProxyServer() {
	proxy := &http.Server{
		Addr: fmt.Sprintf(":%d", GlobalConfig.ProxyPort),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			LogRequest(r, "二次代理")
			if r.Method == http.MethodConnect {
				HandleProxyTunneling(w, r)
			} else {
				HandleProxyHTTP(w, r)
			}
		}),
	}
	log.Printf("二次代理服务器启动在端口 %d", GlobalConfig.ProxyPort)
	log.Fatal(proxy.ListenAndServe())
}

// StartDirectServer 启动直接转发服务器
func StartDirectServer() {
	direct := &http.Server{
		Addr: fmt.Sprintf(":%d", GlobalConfig.DirectPort),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			LogRequest(r, "正向代理")
			if r.Method == http.MethodConnect {
				HandleDirectTunneling(w, r)
			} else {
				HandleDirectHTTP(w, r)
			}
		}),
	}
	log.Printf("直接转发服务器启动在端口 %d", GlobalConfig.DirectPort)
	log.Fatal(direct.ListenAndServe())
}