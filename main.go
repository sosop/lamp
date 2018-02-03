package main

import (
	"lamp/server"
	"os"
	"syscall"
	"os/signal"
	log	"github.com/golang/glog"
	"flag"
)

func init() {
	flag.Parse()
}

func main() {

	// tcp连接请求监听
	go server.Listen()
	// http请求监听
	go server.ListenHttp()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	log.Info("服务启动成功")
	s := <-c
	log.Info(s.String())
	log.Flush()
	os.Exit(0)
}
