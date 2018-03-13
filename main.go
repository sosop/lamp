package main

import (
	"lamp/server"
	"os"
	"syscall"
	"os/signal"
	log	"github.com/golang/glog"
	"flag"
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
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	log.Info("服务启动成功")
	s := <-c
	log.Info(s.String())
	log.Flush()
	os.Exit(0)
}
