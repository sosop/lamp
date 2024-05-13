package server

import (
	"flag"
	"fmt"
	"lamp/utils"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	log "github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	OFFLINE = iota
	DANGER
	ONLINE
	NETWORK         = "tcp"
	DEVIATION       = 3
	LIMITS          = 10240
	OnlineType      = 0
	UnknownlineType = 1
)

var (
	ConnPool   map[string]*TCPConn
	ConnNilErr = errors.New("connection is nil")
	// readType     int8 = 0
	poolMux      sync.Mutex
	noticeDomain = "127.0.0.1:8080"
)

type TCPConn struct {
	Conn          net.Conn `json:"omitempty"`
	HeartInterval int
	RegisterMsg   string
	HeartMsg      string
	Status        int8
	LastHeart     time.Time
	LastCheck     time.Time
	StopCheck     chan struct{}
	Checking      bool
	mux           *sync.Mutex
	sendMux       *sync.Mutex
	Result        chan []byte
}

func init() {
	flag.Parse()
	viper.SetDefault("tcp.host", "0.0.0.0")
	viper.SetDefault("tcp.port", 7777)
	viper.SetDefault("tcp.conns", LIMITS)

	conns := viper.GetInt("tcp.conns")

	if LIMITS < conns {
		panic(fmt.Sprint("支持连接数超过最大限制：", LIMITS, " 配置连接数：", conns))
	}
	ConnPool = make(map[string]*TCPConn, conns)
	noticeDomain = viper.GetString("noticeDomain")
	initPool()
}

func initPool() {
	datas, err := utils.GET(utils.MakeUrl(noticeDomain, "/light/api/dtu/all"))
	if err != nil {
		log.Error(err)
		return
	}

	for _, tc := range datas {
		err = AddToPoole(&TCPConn{RegisterMsg: tc.DeviceCode, HeartMsg: tc.BeatContent, HeartInterval: tc.BeatTime}, UnknownlineType)
		if err != nil {
			log.Error(err)
			continue
		}
	}

}

func Refresh() {
	log.Info("refresh begin")
	poolMux.Lock()
	defer poolMux.Unlock()
	// 清空池
	for k, c := range ConnPool {
		delete(ConnPool, k)
		c.Close()
	}
	// 从新初始化
	initPool()
	log.Info("refresh end")
}

func Listen() {
	host := viper.GetString("tcp.host")
	port := viper.GetInt("tcp.port")
	l, err := net.Listen(NETWORK, fmt.Sprint(host, ":", port))
	if err != nil {
		panic(err)
	}
	defer l.Close()
	cleanupHook()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Error(err)
			continue
		}
		log.Info("设备连接成功", conn.RemoteAddr())
		// 接收请求协程
		err = AddToPoole(&TCPConn{Status: ONLINE, Conn: conn}, OnlineType)
		if err != nil {
			log.Error(err)
		}

	}
}

func AddToPoole(tcpConn *TCPConn, addtype int8) error {
	defer poolMux.Unlock()
	poolMux.Lock()
	log.Info("开始加入池...")
	times := 0
	if addtype == OnlineType && utils.Trim(tcpConn.RegisterMsg) == "" {
		// 设置读超时
		tcpConn.Conn.SetReadDeadline(time.Now().Add(time.Second * 8))
	REREAD:
		if times >= 2 {
			tcpConn.Conn.Close()
			return errors.Errorf("retry times greater than 2")
		}

		// 读取第一条信息就是注册信息
		data, err := readData(tcpConn.Conn)
		if err != nil || len(data) > 35 {
			log.Error("nil err then data length too long", err)
			tcpConn.Conn.Close()
			return err
		}

		if len(data) == 0 {
			times++
			goto REREAD
		}

		// 取消读超时
		tcpConn.Conn.SetReadDeadline(time.Time{})

		tcpConn.RegisterMsg = utils.Trim(string(data))
		log.Info("注册信息：", tcpConn.RegisterMsg)
	}

	// 在池中已存在
	if tc, ok := ConnPool[utils.Trim(tcpConn.RegisterMsg)]; ok {
		if addtype == OnlineType {
			tc.Conn = tcpConn.Conn
			tc.Status = tcpConn.Status
		} else {
			tc.HeartMsg = tcpConn.HeartMsg
			tc.HeartInterval = tcpConn.HeartInterval
		}
	} else {
		tcpConn.StopCheck = make(chan struct{}, 1)
		tcpConn.Result = make(chan []byte, 1)
		ConnPool[utils.Trim(tcpConn.RegisterMsg)] = tcpConn
	}
	if addtype == OnlineType {
		go ConnPool[utils.Trim(tcpConn.RegisterMsg)].handleRequest()
	}
	go ConnPool[utils.Trim(tcpConn.RegisterMsg)].heartbeat()
	log.Info("已完成入池")
	return nil
}

func cleanupHook() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		CloseAllConn()
	}()
}

func (tcpConn *TCPConn) handleRequest() {
	log.Info("开始接收请求")
	for {
		buf, err := readData(tcpConn.Conn)
		if err != nil {
			log.Error(err)
			if err == ConnNilErr || err.Error() == "EOF" {
				return
			}
			continue
		}
		if tcpConn.HeartMsg != "" {
			// 是否是心跳包
			// fmt.Println("->>", utils.Trim(string(buf)), "-->", tcpConn.HeartMsg, utils.Trim(string(buf)) == tcpConn.HeartMsg)
			if utils.Trim(string(buf)) == tcpConn.HeartMsg {
				tcpConn.LastHeart = time.Now()
				utils.SetEX("heart_"+tcpConn.RegisterMsg, tcpConn.HeartInterval+DEVIATION, []byte(tcpConn.HeartMsg))
			} else {
				tcpConn.writeResult(buf)
			}
		}
		// fmt.Println(string(buf), fmt.Sprintf("%X", buf))
		log.Info(tcpConn.RegisterMsg, "-->", string(buf), "--", fmt.Sprintf("%X", buf))
	}
}

func (tcpConn *TCPConn) heartbeat() {
	defer tcpConn.mux.Unlock()
	tcpConn.mux.Lock()

	if tcpConn.HeartInterval <= 0 || utils.Trim(tcpConn.HeartMsg) == "" {
		return
	}

	// 阻塞到tcpConn.Checking == false
	if tcpConn.Checking {
		// tcpConn.stopCheckHeart()
		return
	}

	t := time.NewTicker(time.Duration(tcpConn.HeartInterval) * time.Second)
	defer t.Stop()

	for {
		select {
		case now := <-t.C:
			tcpConn.Checking = true
			tcpConn.LastCheck = now
			heartMsg, err := utils.Get("heart_" + tcpConn.RegisterMsg)
			if err != nil && err != utils.ErrNil {
				log.Error(err)
				continue
			}
			tcpConn.check(heartMsg)
		case <-tcpConn.StopCheck:
			tcpConn.Checking = false
			return
		}
	}

	/**
	// 假如心跳处于危险或死亡状态，则拿到包就检查
	if tcpConn.Status == DANGER || tcpConn.Status == DEAD {
		tcpConn.check(data)
		return
	}

	// 满足间隔及其偏差则进行心跳检测
	interval := time.Now().Sub(tcpConn.LastCheck).Seconds()
	if (0 - DEVIATION) <= interval && DEVIATION >= interval {
		tcpConn.check(data)
	}
	*/
}

func (tcpConn *TCPConn) check(data []byte) {
	// 与心跳内容进行比较
	if data != nil && string(data) == tcpConn.HeartMsg {
		tcpConn.Status = ONLINE
		err := utils.POST(utils.MakeUrl(noticeDomain, "/light/api/dtu/connection/", tcpConn.RegisterMsg, "/", strconv.Itoa(ONLINE)))
		if err != nil {
			log.Error(err)
		}
	} else if tcpConn.Status == ONLINE {
		tcpConn.Status = DANGER
	} else if tcpConn.Status == DANGER {
		tcpConn.Status = OFFLINE
		err := utils.POST(utils.MakeUrl(noticeDomain, "/light/api/dtu/connection/", tcpConn.RegisterMsg, "/", strconv.Itoa(OFFLINE)))
		if err != nil {
			log.Error(err)
		}
	}
}

func readData(conn net.Conn) ([]byte, error) {
	if conn != nil {
		data := make([]byte, 1024)
		n, err := conn.Read(data)
		return data[0:n], err
	}
	return nil, ConnNilErr
}

func SendCMD(tag, cmd string, cmdType int8) ([]byte, error) {
	var err error
	var rData, dst []byte
	if tcpConn, ok := ConnPool[utils.Trim(tag)]; ok {
		defer tcpConn.sendMux.Unlock()
		tcpConn.sendMux.Lock()
		fmt.Sscanf(cmd, "%X", &dst)
		log.Info("转义后：", dst)
		_, err = tcpConn.Conn.Write(dst)
		if err != nil {
			return rData, err
		}
		// log.Info("写入成功：", n)

		rData, err = tcpConn.readResult()

		log.Info("发送指令：", tcpConn.RegisterMsg, "  ", cmd, "  ", dst)
	} else {
		err = errors.New(fmt.Sprint("[", tag, "]不存在，没有此连接信息"))
	}
	return rData, err
}

func CloseConn(tag string) error {
	var err error = nil
	if tcpConn, ok := ConnPool[utils.Trim(tag)]; ok && tcpConn != nil {
		err = tcpConn.Close()
	} else {
		err = errors.New(fmt.Sprint("[", tag, "]不存在，没有此连接信息"))
	}
	return err
}

func (tcpConn *TCPConn) Close() error {
	var err error
	tcpConn.stopCheckHeart()
	if tcpConn.Conn != nil {
		err = tcpConn.Conn.Close()
		tcpConn.Conn = nil
	}
	// close(tcpConn.Destroy)
	delete(ConnPool, utils.Trim(tcpConn.RegisterMsg))
	return err
}

func (tcpConn *TCPConn) stopCheckHeart() {
	select {
	case tcpConn.StopCheck <- struct{}{}:
		log.Info("停止检测心跳")
	case <-time.After(time.Second * 3):
		log.Info("超时")
	}
}

func CloseAllConn() {
	var err error
	for tag := range ConnPool {
		err = CloseConn(tag)
		if err != nil {
			log.Error(tag, "===>", err)
		}
	}
}

func (tcpConn *TCPConn) readResult() ([]byte, error) {
	var result []byte
	var err error
	select {
	case <-time.After(time.Second * 3):
		err = errors.New("读取返回结果超时")
		log.Error(err)
	case result = <-tcpConn.Result:
	}
	return result, err
}

func (tcpConn *TCPConn) writeResult(result []byte) error {
	var err error
	select {
	case <-time.After(time.Second * 3):
		err = errors.New("写入结果结果超时")
		log.Error(err)
	case tcpConn.Result <- result:
	}
	return err
}
