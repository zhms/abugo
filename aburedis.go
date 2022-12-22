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

func (c *AbuRedis) Get(key string) interface{} {
	conn := c.redispool.Get()
	defer conn.Close()
	ret, err := conn.Do("get", key)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	return ret
}

func (c *AbuRedis) Set(key string, value interface{}) error {
	conn := c.redispool.Get()
	defer conn.Close()
	output, _ := json.Marshal(&value)
	_, err := conn.Do("set", key, output)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) SetString(key string, value string) error {
	conn := c.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("set", key, value)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) SetEx(key string, expire int, value interface{}) error {
	conn := c.redispool.Get()
	defer conn.Close()
	output, _ := json.Marshal(&value)
	_, err := conn.Do("setex", key, expire, string(output))
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}
func (c *AbuRedis) SetStringEx(key string, expire int, value string) error {
	conn := c.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("setex", key, expire, value)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) Del(key string) error {
	conn := c.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("del", key)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) Expire(key string, expire int) error {
	conn := c.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("expire", key, expire)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) HSet(key string, field string, value interface{}) error {
	conn := c.redispool.Get()
	defer conn.Close()
	output, _ := json.Marshal(&value)
	_, err := conn.Do("hset", key, field, string(output))
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) HSetString(key string, field string, value string) error {
	conn := c.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("hset", key, field, value)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) HGet(key string, field string) interface{} {
	conn := c.redispool.Get()
	defer conn.Close()
	ret, err := conn.Do("hget", key, field)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	return ret
}

func (c *AbuRedis) HDel(key string, field string) error {
	conn := c.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("hdel", key, field)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	return nil
}

func (c *AbuRedis) HKeys(key string) []string {
	conn := c.redispool.Get()
	defer conn.Close()
	keys, err := conn.Do("hkeys", key)
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

func (c *AbuRedis) SAdd(key string, value interface{}) error {
	conn := c.redispool.Get()
	defer conn.Close()
	output, _ := json.Marshal(&value)
	_, err := conn.Do("sadd", key, string(output))
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	return nil
}

func (c *AbuRedis) SAddString(key string, value string) error {
	conn := c.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("sadd", key, value)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	return nil
}

