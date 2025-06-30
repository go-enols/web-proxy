package main

import (
	"flag"
	"strings"
)

// Config 代理服务器配置
type Config struct {
	ProxyPort  int    // 用于二次代理转发的端口
	DirectPort int    // 用于直接转发的端口
	ProxyURL   string // 第二级代理服务器URL
	AuthUser   string // 代理服务器认证用户名
	AuthPass   string // 代理服务器认证密码
}

// GlobalConfig 全局配置实例
var GlobalConfig Config

// InitConfig 初始化配置
func InitConfig() {
	flag.IntVar(&GlobalConfig.ProxyPort, "proxy-port", 9522, "用于二次代理转发的监听端口")
	flag.IntVar(&GlobalConfig.DirectPort, "direct-port", 9521, "用于直接转发的监听端口")
	flag.StringVar(&GlobalConfig.ProxyURL, "proxy-url", "", "第二级代理服务器URL,支持认证格式: user:pwd@ip:port 或 ip:port")
	flag.StringVar(&GlobalConfig.AuthUser, "user", "", "代理服务器认证用户名")
	flag.StringVar(&GlobalConfig.AuthPass, "pwd", "", "代理服务器认证密码")
	flag.Parse()
}

// ParseProxyURL 解析代理服务器的URL，提取认证信息，并返回代理地址和认证头
// PS: 注意该解析只能解析 账户:密码@服务器:端口
func ParseProxyURL(proxyURL string) (string, string, error) {
	data := strings.Split(proxyURL, "@")
	var (
		user   string
		server string
	)
	if len(data) == 2 {
		user = data[0]
		server = data[1]
	} else if len(data) == 1 {
		server = data[0]
	}
	return server, user, nil
}