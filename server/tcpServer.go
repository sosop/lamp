package server

import (
	"net"
	log	"github.com/golang/glog"
	"fmt"
	"io/ioutil"
	"github.com/pkg/errors"
	"time"
	"lamp/utils"
	"strings"
	"github.com/spf13/viper"
	"os"
	"os/signal"
	"syscall"
)

const (
	NETWORK 		= "tcp"
	DEVIATION		= 3
	LIMITS			= 10240
	OFFLINE			= iota
	DANGER
	ONLINE
)

type CMDType int8

var (
	ConnPool 			map[string]*TCPConn
	ConnNilErr			= errors.New("connection is nil")
	readType CMDType 	= 0
)

type TCPConn struct {
	Conn 			net.Conn
	HeartInterval 	int
	RegisterMsg		string
	HeartMsg		string
	Status			int8
	LastHeart		time.Time		`json:"omitempty"`
	LastCheck		time.Time		`json:"omitempty"`
	Destroy			chan struct{}	`json:"omitempty"`
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
}

func Listen() {
	host := viper.GetString("tcp.host")
	port := viper.GetInt("tcp.port")
	l, err := net.Listen(NETWORK, fmt.Sprint(host, ":", port))
	if err != nil {
		panic(err)
	}
	cleanupHook()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Error(err)
			continue
		}
		log.Info("设备连接成功")

		// 接收请求协程
		tcpConn := TCPConn{Status: ONLINE, Conn: conn}
		tcpConn.Destroy = make(chan struct{}, 1)
		go handleRequest(&tcpConn)
	}
}

func addPoolOfOnLine(tcpConn TCPConn) {
	if tc, ok := ConnPool[tcpConn.RegisterMsg]; ok {
		tc.Conn = tcpConn.Conn
		tcpConn.Status = ONLINE
	} else {
		ConnPool[tcpConn.RegisterMsg] = &tcpConn
	}
	fmt.Println(len(ConnPool))
}

func cleanupHook() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	go func() {
		<-c
		CloseAllConn()
	}()
}

func handleRequest(tcpConn *TCPConn) {
	for {
		log.Info("go---->")
		fmt.Println("go---->", tcpConn.Conn.RemoteAddr())
		buf, err := readData(tcpConn.Conn)
		log.Info("go---->", string(buf))
		if err != nil {
			log.Error(err)
			if err == ConnNilErr {
				tcpConn.destroy()
				return
			}
			continue
		}
		// 是否是心跳包
		if strings.TrimSpace(string(buf)) == tcpConn.HeartMsg {
			tcpConn.LastHeart = time.Now()
			utils.SetEX("heart_" + tcpConn.RegisterMsg, tcpConn.HeartInterval + DEVIATION, []byte(tcpConn.HeartMsg))
		}
		// fmt.Println(string(buf), fmt.Sprintf("%X", buf))
		log.Info(string(buf), "--", fmt.Sprintf("%X", buf))
	}
}

func (tcpConn *TCPConn) heartbeat() {

	t := time.Tick(time.Duration(tcpConn.HeartInterval) * time.Second)

	for {
		select {
		case now := <- t:
			tcpConn.LastCheck = now
			heartMsg, err := utils.Get("heart_" + tcpConn.RegisterMsg)
			if err != nil {
				log.Error(err)
				continue
			}
			tcpConn.check(heartMsg)
		case <- tcpConn.Destroy:
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
	} else if tcpConn.Status == ONLINE {
		tcpConn.Status = DANGER
	} else if tcpConn.Status == DANGER {
		tcpConn.Status = OFFLINE
		//  todo 通知管理端，设备已掉线
	}
}

func readData(conn net.Conn) ([]byte, error) {
	if conn != nil {
		return ioutil.ReadAll(conn)

		// data := make([]byte, 0, 128)
		// _, err := conn.Read(data)
		// fmt.Println("====->", string(data))
		// return data, err

	}
	return nil, ConnNilErr
}

func SendCMD(tag, cmd string, cmdType CMDType) ([]byte, error) {
	var err error = nil
	var rData, dst []byte
	if tcpConn, ok := ConnPool[tag]; ok {
		fmt.Sscanf(cmd, "%X", &dst)
		_, err = tcpConn.Conn.Write(dst)
		if cmdType == readType {
			rData, err = readData(tcpConn.Conn)
		}
	} else {
		err = errors.New(fmt.Sprint("[", tag, "]不存在，没有此连接信息"))
	}
	return rData, err
}

func CloseConn(tag string) error {
	var err error = nil
	if tcpConn, ok := ConnPool[tag]; ok && tcpConn != nil {
		err = tcpConn.Close()
	} else {
		err = errors.New(fmt.Sprint("[", tag, "]不存在，没有此连接信息"))
	}
	return err
}

func (tcpConn *TCPConn) Close() error {
	var err error
	tcpConn.destroy()
	if tcpConn.Conn != nil {
		err = tcpConn.Conn.Close()
		tcpConn.Conn = nil
	}
	// close(tcpConn.Destroy)
	delete(ConnPool, tcpConn.RegisterMsg)
	return err
}

func (tcpConn *TCPConn) destroy() {
	select {
	case tcpConn.Destroy <- struct{}{}:
		log.Info("注销成功")
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