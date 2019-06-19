package utils

import (
	"github.com/garyburd/redigo/redis"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	_ "lamp/config"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	Pool   *redis.Pool
	ErrNil = redis.ErrNil
	//transport = &http.Transport{
	//	DialContext: (net.Dialer{
	//		KeepAlive: 1 * time.Hour,
	//	}).DialContext,
	//	DisableKeepAlives: false,
	//}
	Client = http.DefaultClient
)

func init() {
	// 设置默认值
	viper.SetDefault("redis.addr", ":6379")
	viper.SetDefault("redis.readTimeout", 3)
	viper.SetDefault("redis.writeTimeout", 6)
	viper.SetDefault("redis.connectTimeout", 6)
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.maxIdle", 6)
	viper.SetDefault("redis.maxActive", 30)
	viper.SetDefault("redis.idleTimeout", 300)

	redisConfig := &RedisConfig{}
	viper.UnmarshalKey("redis", redisConfig)
	Pool = newPool(redisConfig)
	cleanupHook()
}

type RedisConfig struct {
	Addr           string
	ReadTimeout    int
	WriteTimeout   int
	ConnectTimeout int
	DB             int
	MaxIdle        int
	MaxActive      int
	IdleTimeout    int
}

func newPool(redisConfig *RedisConfig) *redis.Pool {

	return &redis.Pool{
		MaxIdle:     redisConfig.MaxIdle,
		MaxActive:   redisConfig.MaxActive,
		IdleTimeout: time.Duration(redisConfig.IdleTimeout) * time.Second,

		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", redisConfig.Addr,
				redis.DialReadTimeout(time.Duration(redisConfig.ReadTimeout)*time.Second),
				redis.DialWriteTimeout(time.Duration(redisConfig.WriteTimeout)*time.Second),
				redis.DialConnectTimeout(time.Duration(redisConfig.ConnectTimeout)*time.Second),
				redis.DialDatabase(redisConfig.DB))
			if err != nil {
				return nil, err
			}
			return c, err
		},

		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

func cleanupHook() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	go func() {
		<-c
		Pool.Close()
		// os.Exit(0)
	}()
}

func Ping() (string, error) {
	conn := Pool.Get()
	defer conn.Close()
	return redis.String(conn.Do("PING"))
}

func Get(key string) ([]byte, error) {
	conn := Pool.Get()
	defer conn.Close()
	return redis.Bytes(conn.Do("GET", key))

}

func Set(key string, value []byte) error {
	conn := Pool.Get()
	defer conn.Close()
	ok, err := redis.String(conn.Do("SET", key, value))
	if err != nil {
		return err
	}
	if "OK" != strings.ToUpper(strings.TrimSpace(ok)) {
		return errors.New("set failur result is " + ok)
	}
	return nil
}

func SetEX(key string, seconds int, value []byte) error {
	conn := Pool.Get()
	defer conn.Close()
	ok, err := redis.String(conn.Do("SETEX", key, seconds, value))
	if err != nil {
		return err
	}
	if "OK" != strings.ToUpper(strings.TrimSpace(ok)) {
		return errors.New("set failur result is " + ok)
	}
	return nil
}

func Exists(key string) (bool, error) {
	conn := Pool.Get()
	defer conn.Close()
	ok, err := redis.Bool(conn.Do("EXISTS", key))
	return ok, err
}

func Delete(key string) (int, error) {
	conn := Pool.Get()
	defer conn.Close()
	return redis.Int(conn.Do("DEL", key))
}
