package abugo

import (
	"encoding/json"
	"fmt"
	"reflect"
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

func (c *AbuRedis) getcallback(channel string) AbuRedisSubCallback {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.subscribecallbacks[channel]
}

func (c *AbuRedis) subscribe(channels ...string) {
	c.pubconnection.Subscribe(redis.Args{}.AddFlat(channels)...)
	if !c.recving {
		go func() {
			for {
				imsg := c.pubconnection.Receive()
				msgtype := reflect.TypeOf(imsg).Name()
				if msgtype == "Message" {
					msg := imsg.(redis.Message)
					callback := c.getcallback(msg.Channel)
					if callback != nil {
						callback(string(msg.Data))
					}
				}
			}
		}()
	}
}

func (c *AbuRedis) Subscribe(channel string, callback AbuRedisSubCallback) {
	c.mu.Lock()
	c.subscribecallbacks[channel] = callback
	c.mu.Unlock()
	c.subscribe(channel)
}

func (c *AbuRedis) Publish(k, v interface{}) error {
	conn := c.redispool.Get()
	defer conn.Close()
	output, _ := json.Marshal(&v)
	_, err := conn.Do("publish", k, output)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) Get(k string) interface{} {
	conn := c.redispool.Get()
	defer conn.Close()
	ret, err := conn.Do("get", k)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	return ret
}

func (c *AbuRedis) Set(k string, v interface{}) error {
	conn := c.redispool.Get()
	defer conn.Close()
	output, _ := json.Marshal(&v)
	_, err := conn.Do("set", k, output)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) SetString(k string, v string) error {
	conn := c.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("set", k, v)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) SetEx(k string, to int, v interface{}) error {
	conn := c.redispool.Get()
	defer conn.Close()
	output, _ := json.Marshal(&v)
	_, err := conn.Do("setex", k, to, string(output))
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}
func (c *AbuRedis) SetStringEx(k string, to int, v string) error {
	conn := c.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("setex", k, to, v)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) Del(k string) error {
	conn := c.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("del", k)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) Expire(k string, to int) error {
	conn := c.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("expire", k, to)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) HSet(k string, f string, v interface{}) error {
	conn := c.redispool.Get()
	defer conn.Close()
	output, _ := json.Marshal(&v)
	_, err := conn.Do("hset", k, f, string(output))
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) HSetString(k string, f string, v string) error {
	conn := c.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("hset", k, f, v)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) HGet(k string, f string) interface{} {
	conn := c.redispool.Get()
	defer conn.Close()
	ret, err := conn.Do("hget", k, f)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	return ret
}

func (c *AbuRedis) HDel(k string, f string) error {
	conn := c.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("hdel", k, f)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	return nil
}

func (c *AbuRedis) HKeys(k string) []string {
	conn := c.redispool.Get()
	defer conn.Close()
	keys, err := conn.Do("hkeys", k)
	ikeys := keys.([]interface{})
	strkeys := []string{}
	if err != nil {
		logs.Error(err.Error())
		return strkeys
	}
	for i := 0; i < len(ikeys); i++ {
		strkeys = append(strkeys, string(ikeys[i].([]byte)))
	}
	return strkeys
}

func (c *AbuRedis) SAdd(k string, f string) error {
	conn := c.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("sadd", k, f)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	return nil
}
