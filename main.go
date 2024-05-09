package main

import (
	"flag"
	"lamp/server"
	"os"
	"os/signal"
	"syscall"

	log "github.com/golang/glog"
	"github.com/spf13/viper"
)

func init() {
	flag.Parse()
	viper.SetDefault("mode", "prod")
}

func main() {

	// tcp连接请求监听
	go server.Listen()
	// http请求监听
	go server.ListenHttp()

	// 监控
	go server.Monitor()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	log.Info("服务启动成功")
	s := <-c
	log.Info(s.String())
	log.Flush()
	os.Exit(0)
}
