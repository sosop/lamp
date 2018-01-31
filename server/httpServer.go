package server

import (
	"github.com/plimble/ace"
	log	"github.com/golang/glog"
	"strings"
	_ "lamp/config"
	"github.com/spf13/viper"
	"fmt"
)

func init() {
	viper.SetDefault("http.host", "0.0.0.0")
	viper.SetDefault("http.port", 9393)
}

func ListenHttp() {
	a := ace.New()
	g := a.Group("/device", func (c *ace.C) {
		log.Info(c.Request.RemoteAddr, c.Request.URL)
		c.Next()
	})
	g.GET("/:tag", device)
	g.DELETE("/:tag", destroy)
	g.POST("/modify", modify)
	g.POST("/create", create)
	g.POST("/command", command)

	a.Run(fmt.Sprint(viper.GetString("http.host"), ":", viper.GetInt("http.port")))
}

type Response struct {
	Code int
	Data interface{}
}

func NewResponse(code int, Data interface{}) Response {
	return Response{code, Data}
}

func create(c *ace.C) {
	tcpConn := TCPConn{}
	c.ParseJSON(&tcpConn)
	if strings.TrimSpace(tcpConn.RegisterMsg) == "" ||
		strings.TrimSpace(tcpConn.HeartMsg) == "" ||
		tcpConn.HeartInterval == 0 {
		c.JSON(400, NewResponse(-1, "参数不正确"))
		return
	}
	if tConn, ok := ConnPool[tcpConn.RegisterMsg]; ok {
		tConn.HeartInterval = tcpConn.HeartInterval
		tConn.HeartMsg = tcpConn.HeartMsg
	} else {
		ConnPool[tcpConn.RegisterMsg] = &tcpConn
		tcpConn.Destroy = make(chan struct{}, 1)
	}
	c.JSON(200, NewResponse(0, "创建成功"))
}

func destroy(c *ace.C) {
	tag := strings.TrimSpace(c.Param("tag"))
	if tcpConn, ok := ConnPool[tag]; ok {
		tcpConn.Close()
	}
	c.JSON(200, NewResponse(0, "删除成功"))
}

func modify(c *ace.C) {
	tcpConn := TCPConn{}
	c.ParseJSON(&tcpConn)

	if strings.TrimSpace(tcpConn.RegisterMsg) == "" {
		c.JSON(400, NewResponse(-1, "参数不正确"))
		return
	}
	if tConn, ok := ConnPool[tcpConn.RegisterMsg]; ok {
		if strings.TrimSpace(tcpConn.HeartMsg) != "" {
			tConn.HeartMsg = tcpConn.HeartMsg
		}
		if tcpConn.HeartInterval > 0 {
			tConn.HeartInterval = tcpConn.HeartInterval
		}
	}
	c.JSON(200, NewResponse(0, "修改成功"))
}

func device(c *ace.C) {
	tag := strings.TrimSpace(c.Param("tag"))
	if strings.ToLower(tag) == "all" {
		c.JSON(200, NewResponse(0, ConnPool))
		return
	}

	if tcpConn, ok := ConnPool[tag]; ok {
		c.JSON(200, NewResponse(0, tcpConn))
		return
	}

	c.JSON(404, nil)
}

func command(c *ace.C) {
	tag := strings.TrimSpace(c.MustPostString("registerMsg", ""))
	cmd := strings.TrimSpace(c.MustPostString("cmd", ""))
	cmdType := c.MustPostInt("cmdType", 0)
	if cmd == "" {
		log.Error(fmt.Errorf("输入指令不正确"))
		c.JSON(400, NewResponse(-1, "请输入正确的指令"))
		return
	}
	data, err := SendCMD(tag, cmd, CMDType(cmdType))
	if err != nil {
		log.Error(err)
		c.JSON(500, NewResponse(-1, err.Error()))
		return
	}
	dataHex := ""
	if data != nil {
		dataHex = fmt.Sprintf("%X", data)
	}
	c.JSON(200, NewResponse(0, dataHex))
}