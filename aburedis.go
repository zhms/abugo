package abugo

import (
	"fmt"
	"sync"
	"time"

	"github.com/beego/beego/logs"
	"github.com/garyburd/redigo/redis"
)

type AbuRedisSubCallback func(string)
type AbuRedis struct {
	redispool          *redis.Pool
	pubconnection      *redis.PubSubConn
	host               string
	port               int
	db                 int
	password           string
	recving            bool
	subscribecallbacks map[string]AbuRedisSubCallback
	mu                 *sync.RWMutex
}

func (c *AbuRedis) Init(prefix string) {
	if c.redispool != nil {
		return
	}
	host := GetConfigString(fmt.Sprint(prefix, ".host"), true, "")
	port := GetConfigInt(fmt.Sprint(prefix, ".port"), true, 0)
	db := GetConfigInt(fmt.Sprint(prefix, ".db"), true, -1)
	password := GetConfigString(fmt.Sprint(prefix, ".password"), true, "")
	maxidle := GetConfigInt(fmt.Sprint(prefix, ".maxidle"), true, 0)
	maxactive := GetConfigInt(fmt.Sprint(prefix, ".maxactive"), true, 0)
	idletimeout := GetConfigInt(fmt.Sprint(prefix, ".idletimeout"), true, 0)
	c.redispool = &redis.Pool{
		MaxIdle:     maxidle,
		MaxActive:   maxactive,
		IdleTimeout: time.Duration(idletimeout) * time.Second,
		Wait:        true,
		Dial: func() (redis.Conn, error) {
			con, err := redis.Dial("tcp", fmt.Sprint(host, ":", port),
				redis.DialPassword(password),
				redis.DialDatabase(db),
			)
			if err != nil {
				logs.Error(err)
				panic(err)
			}
			return con, nil
		},
	}
	conn, err := redis.Dial("tcp", fmt.Sprint(host, ":", port),
		redis.DialPassword(password),
		redis.DialDatabase(db),
	)
	if err != nil {
		logs.Error(err)
		panic(err)
	}
	c.pubconnection = new(redis.PubSubConn)
	c.pubconnection.Conn = conn
	c.recving = false
	c.subscribecallbacks = make(map[string]AbuRedisSubCallback)
	c.mu = new(sync.RWMutex)
	logs.Debug("连接redis 成功:", host, port, db)
}
