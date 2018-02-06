package server

import (
	"net"
	log	"github.com/golang/glog"
	"fmt"
	"github.com/pkg/errors"
	"time"
	"lamp/utils"
	"github.com/spf13/viper"
	"os"
	"os/signal"
	"syscall"
	"sync"
	"strconv"
)

const (
	OFFLINE			= iota
	DANGER
	ONLINE
	NETWORK 		= "tcp"
	DEVIATION		= 3
	LIMITS			= 10240
	OnlineType		= 0
	UnknownlineType	= 1
)

var (
	ConnPool 			map[string]*TCPConn
	ConnNilErr			= errors.New("connection is nil")
	readType int8 		= 0
	poolMux				sync.Mutex
	noticeDomain		= "127.0.0.1:8080"
)

type TCPConn struct {
	Conn 			net.Conn		`json:"omitempty"`
	HeartInterval 	int
	RegisterMsg		string
	HeartMsg		string
	Status			int8
	LastHeart		time.Time
	LastCheck		time.Time
	StopCheck		chan struct{}	`json:"omitempty"`
	Checking		bool
	mux				sync.Mutex
	Result			chan []byte		`json:"omitempty"`
}

func init() {

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
		panic(err)
	}

	for _, tc := range datas {
		err = AddToPoole(TCPConn{RegisterMsg:tc.DeviceCode, HeartMsg:tc.BeatContent, HeartInterval:tc.BeatTime}, UnknownlineType)
		if err != nil {
			log.Error(err)
			continue
		}
	}

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
		err = AddToPoole(TCPConn{Status: ONLINE, Conn: conn}, OnlineType)
		if err != nil {
			log.Error(err)
			continue
		}
	}
}

func AddToPoole(tcpConn TCPConn, addtype int8) error {
	defer poolMux.Unlock()
	poolMux.Lock()
	log.Info("开始加入池...")
	if addtype == OnlineType && utils.Trim(tcpConn.RegisterMsg) == "" {
		// 读取第一条信息就是注册信息
	REREAD:
		data, err := readData(tcpConn.Conn)

		if len(data) == 0 {
			goto REREAD
		}

		if err != nil || len(data) > 35 {
			log.Error("nil err then data length too long", err)
			tcpConn.Conn.Close()
			return err
		}
		tcpConn.RegisterMsg = utils.Trim(string(data))
		log.Info("注册信息：", tcpConn.RegisterMsg)
	}

	// 在池中已存在
	var tConn *TCPConn
	if tc, ok := ConnPool[utils.Trim(tcpConn.RegisterMsg)]; ok {
		if addtype == OnlineType {
			tc.Conn = tcpConn.Conn
			tc.Status = tcpConn.Status
		} else {
			tc.HeartMsg = tcpConn.HeartMsg
			tc.HeartInterval = tcpConn.HeartInterval
		}
		tConn = tc

	} else {
		tcpConn.StopCheck = make(chan struct{}, 1)
		tcpConn.Result = make(chan []byte, 1)
		ConnPool[utils.Trim(tcpConn.RegisterMsg)] = &tcpConn
		tConn = ConnPool[utils.Trim(tcpConn.RegisterMsg)]
	}
	if addtype == OnlineType {
		go tConn.handleRequest()
	}
	go tConn.heartbeat()
	log.Info("已完成入池")
	return nil
}

func cleanupHook() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	go func() {
		<-c
		CloseAllConn()
	}()
}

func (tcpConn *TCPConn) handleRequest() {
	// 添加到池中
	/*
	err := AddToPoole(*tcpConn, OnlineType)
	if err != nil {
		log.Error(err)
		return
	}
	*/
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
		tcpConn.stopCheckHeart()
	}

	t := time.Tick(time.Duration(tcpConn.HeartInterval) * time.Second)

	for {
		select {
		case now := <- t:
			tcpConn.Checking = true
			tcpConn.LastCheck = now
			heartMsg, err := utils.Get("heart_" + tcpConn.RegisterMsg)
			if err != nil && err != utils.ErrNil {
				log.Error(err)
				continue
			}
			tcpConn.check(heartMsg)
		case <- tcpConn.StopCheck:
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
		_, err := utils.POST(utils.MakeUrl(noticeDomain, "/light/api/dtu/connection/", tcpConn.RegisterMsg, "/", strconv.Itoa(ONLINE)))
		if err != nil {
			log.Error(err)
		}
	} else if tcpConn.Status == ONLINE {
		tcpConn.Status = DANGER
	} else if tcpConn.Status == DANGER {
		tcpConn.Status = OFFLINE
		_, err := utils.POST(utils.MakeUrl(noticeDomain, "/light/api/dtu/connection/", tcpConn.RegisterMsg, "/", strconv.Itoa(OFFLINE)))
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
	var err error = nil
	var rData, dst []byte
	if tcpConn, ok := ConnPool[utils.Trim(tag)]; ok {
		fmt.Sscanf(cmd, "%X", &dst)
		_, err = tcpConn.Conn.Write(dst)
		if cmdType == readType {
			rData, err = tcpConn.readResult()
		}
		log.Info("发送指令：", tcpConn.RegisterMsg, "  ", cmd, "  ", string(dst))
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
	case <- time.After(time.Second * 3):
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
	case <- time.After(time.Second * 3):
		err = errors.New("读取返回结果超时")
		log.Error(err)
	case result = <- tcpConn.Result:
	}
	return result, err
}

func (tcpConn *TCPConn) writeResult(result []byte) error {
	var err error
	select {
	case <- time.After(time.Second * 3):
		err = errors.New("写入结果结果超时")
		log.Error(err)
	case tcpConn.Result <- result:
	}
	return err
}