package main

import (
	"log"
)

func main() {
	// 初始化配置
	InitConfig()

	// 打印启动信息
	log.Println("Web Proxy Server 启动中...")
	log.Printf("二次代理端口: %d", GlobalConfig.ProxyPort)
	log.Printf("直接转发端口: %d", GlobalConfig.DirectPort)
	if GlobalConfig.ProxyURL != "" {
		log.Printf("上级代理: %s", GlobalConfig.ProxyURL)
	}
	if GlobalConfig.AuthUser != "" {
		log.Printf("认证用户: %s", GlobalConfig.AuthUser)
	}

	// 启动二次代理服务器
	go StartProxyServer()

	// 启动直接转发服务器
	go StartDirectServer()

	// 阻塞主goroutine
	select {}
}
