package main

import (
	"io"
	"log"
	"net/http"
)

// Transfer 转发数据
func Transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}

// LogRequest Log日志
func LogRequest(r *http.Request, title string) {
	log.Printf("[%s] 请求: %s %s %s", title, r.Method, r.Host, r.RequestURI)
	if r.TLS != nil {
		log.Println("[" + title + "] 安全连接: TLS已启用")
	} else {
		log.Println("[" + title + "]安全连接: TLS未启用")
	}
}
