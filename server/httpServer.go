package server

import (
	"fmt"
	_ "lamp/config"
	"lamp/utils"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/golang/glog"
	"github.com/spf13/viper"
)

func init() {
	viper.SetDefault("http.host", "0.0.0.0")
	viper.SetDefault("http.port", 9393)
}

func ListenHttp() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	g := r.Group("/device")
	g.GET("/:tag", device)
	g.DELETE("/:tag", destroy)
	g.POST("/modify", modify)
	g.POST("/create", create)
	g.POST("/command", command)
	g.POST("/refresh", refresh)

	r.Run(fmt.Sprint(viper.GetString("http.host"), ":", viper.GetInt("http.port")))
}

type Response struct {
	Code int
	Data interface{}
}

func NewResponse(code int, data interface{}) Response {
	return Response{code, data}
}

func create(c *gin.Context) {
	tcpConn := TCPConn{}
	if err := c.ShouldBindJSON(&tcpConn); err != nil {
		c.JSON(http.StatusBadRequest, NewResponse(-1, "json 参数格式错误"))
		return
	}
	if utils.Trim(tcpConn.RegisterMsg) == "" ||
		utils.Trim(tcpConn.HeartMsg) == "" ||
		tcpConn.HeartInterval == 0 {
		c.JSON(http.StatusBadRequest, NewResponse(-1, "参数不正确"))
		return
	}
	AddToPoole(&tcpConn, UnknownlineType)
	c.JSON(http.StatusOK, NewResponse(0, "创建成功"))
}

func destroy(c *gin.Context) {
	tag := utils.Trim(c.Param("tag"))
	if tcpConn, ok := ConnPool[tag]; ok {
		tcpConn.Close()
	}
	c.JSON(200, NewResponse(0, "删除成功"))
}

func modify(c *gin.Context) {
	tcpConn := TCPConn{}
	if err := c.ShouldBindJSON(&tcpConn); err != nil {
		c.JSON(http.StatusBadRequest, NewResponse(-1, "json 参数格式错误"))
		return
	}
	updated := false
	if utils.Trim(tcpConn.RegisterMsg) == "" {
		c.JSON(400, NewResponse(-1, "参数不正确"))
		return
	}
	if tConn, ok := ConnPool[tcpConn.RegisterMsg]; ok {
		heartMsg := utils.Trim(tcpConn.HeartMsg)
		if heartMsg != "" && heartMsg != tConn.HeartMsg {
			tConn.HeartMsg = tcpConn.HeartMsg
			updated = true
		}
		if tcpConn.HeartInterval > 0 && tcpConn.HeartInterval != tConn.HeartInterval {
			tConn.HeartInterval = tcpConn.HeartInterval
			updated = true
		}
		if updated {
			go tConn.heartbeat()
		}
	}
	c.JSON(200, NewResponse(0, "修改成功"))
}

func device(c *gin.Context) {
	tag := utils.Trim(c.Param("tag"))
	if strings.ToLower(tag) == "all" {
		c.JSON(http.StatusOK, NewResponse(0, ConnPool))
		return
	}

	if tcpConn, ok := ConnPool[tag]; ok {
		c.JSON(http.StatusOK, NewResponse(0, tcpConn))
		return
	}

	c.JSON(http.StatusNotFound, nil)
}

func command(c *gin.Context) {
	tag := utils.Trim(c.PostForm("registerMsg"))
	cmd := utils.Trim(c.PostForm("cmd"))
	cmdType := c.PostForm("cmdType")
	cmdTypeInt := 0
	var err error
	if cmdType != "" {
		cmdTypeInt, err = strconv.Atoi(cmdType)
	}
	if err != nil {
		log.Error(err)
		c.JSON(http.StatusBadRequest, NewResponse(-1, "cmdType 格式不正确"))
		return
	}
	if cmd == "" {
		log.Error(fmt.Errorf("输入指令不正确"))
		c.JSON(http.StatusBadRequest, NewResponse(-1, "请输入正确的指令"))
		return
	}
	log.Info("指令状态：", cmdType)
	data, err := SendCMD(tag, cmd, int8(cmdTypeInt))
	if err != nil {
		log.Error(err)
		c.JSON(http.StatusInternalServerError, NewResponse(-1, err.Error()))
		return
	}
	dataHex := ""
	if data != nil {
		dataHex = fmt.Sprintf("%X", data)
	}
	c.JSON(http.StatusOK, NewResponse(0, dataHex))
}

func refresh(c *gin.Context) {
	Refresh()
	c.JSON(http.StatusOK, "refresh ok!")
}
