package abugo

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
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

func (this *AbuRedis) Init(prefix string) {
	if this.redispool != nil {
		return
	}
	host := GetConfigString(fmt.Sprint(prefix, ".host"), true, "")
	port := GetConfigInt(fmt.Sprint(prefix, ".port"), true, 0)
	db := GetConfigInt(fmt.Sprint(prefix, ".db"), true, -1)
	password := GetConfigString(fmt.Sprint(prefix, ".password"), true, "")
	maxidle := GetConfigInt(fmt.Sprint(prefix, ".maxidle"), true, 0)
	maxactive := GetConfigInt(fmt.Sprint(prefix, ".maxactive"), true, 0)
	idletimeout := GetConfigInt(fmt.Sprint(prefix, ".idletimeout"), true, 0)
	this.redispool = &redis.Pool{
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
	this.pubconnection = new(redis.PubSubConn)
	this.pubconnection.Conn = conn
	this.recving = false
	this.subscribecallbacks = make(map[string]AbuRedisSubCallback)
	this.mu = new(sync.RWMutex)
	logs.Debug("连接redis 成功:", host, port, db)
}

func (this *AbuRedis) getcallback(channel string) AbuRedisSubCallback {
	this.mu.Lock()
	defer this.mu.Unlock()
	return this.subscribecallbacks[channel]
}

func (this *AbuRedis) subscribe(channels ...string) {
	this.pubconnection.Subscribe(redis.Args{}.AddFlat(channels)...)
	if !this.recving {
		go func() {
			for {
				imsg := this.pubconnection.Receive()
				msgtype := reflect.TypeOf(imsg).Name()
				if msgtype == "Message" {
					msg := imsg.(redis.Message)
					callback := this.getcallback(msg.Channel)
					if callback != nil {
						callback(string(msg.Data))
					}
				}
			}
		}()
	}
}

func (this *AbuRedis) Subscribe(channel string, callback AbuRedisSubCallback) {
	this.mu.Lock()
	this.subscribecallbacks[channel] = callback
	this.mu.Unlock()
	this.subscribe(channel)
}

func (this *AbuRedis) Publish(k, v interface{}) error {
	conn := this.redispool.Get()
	defer conn.Close()
	output, _ := json.Marshal(&v)
	_, err := conn.Do("publish", k, output)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (this *AbuRedis) Get(key string) interface{} {
	conn := this.redispool.Get()
	defer conn.Close()
	ret, err := conn.Do("get", key)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	return ret
}

type RedisGetExCallback func() (int, interface{})

func (this *AbuRedis) GetEx(key string, callback RedisGetExCallback) string {
	redisdata := this.Get(key)
	if redisdata != nil {
		return string(redisdata.([]byte))
	} else {
		t, data := callback()
		this.SetEx(key, t, data)
		jdata := data.(string)
		this.SetStringEx(key, t, jdata)
		return jdata
	}
}

func (this *AbuRedis) GetExMap(key string, callback RedisGetExCallback) *map[string]interface{} {
	redisdata := this.Get(key)
	if redisdata != nil {
		jdata := make(map[string]interface{})
		json.Unmarshal(redisdata.([]byte), &jdata)
		return &jdata
	} else {
		t, data := callback()
		this.SetEx(key, t, data)
		jdata := data.(*map[string]interface{})
		return jdata
	}
}

func (this *AbuRedis) GetExArray(key string, callback RedisGetExCallback) *[]interface{} {
	redisdata := this.Get(key)
	if redisdata != nil {
		jdata := []interface{}{}
		json.Unmarshal(redisdata.([]byte), &jdata)
		return &jdata
	} else {
		t, data := callback()
		this.SetEx(key, t, data)
		jdata := data.(*[]interface{})
		return jdata
	}
}

func (this *AbuRedis) Set(key string, value interface{}) error {
	conn := this.redispool.Get()
	defer conn.Close()
	output, _ := json.Marshal(&value)
	_, err := conn.Do("set", key, output)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (this *AbuRedis) SetString(key string, value string) error {
	conn := this.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("set", key, value)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (this *AbuRedis) SetEx(key string, expire int, value interface{}) error {
	conn := this.redispool.Get()
	defer conn.Close()
	output, _ := json.Marshal(&value)
	_, err := conn.Do("setex", key, expire, string(output))
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}
func (this *AbuRedis) SetStringEx(key string, expire int, value string) error {
	conn := this.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("setex", key, expire, value)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (this *AbuRedis) Del(key string) error {
	conn := this.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("del", key)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (this *AbuRedis) Expire(key string, expire int) error {
	conn := this.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("expire", key, expire)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (this *AbuRedis) HSet(key string, field string, value interface{}) error {
	conn := this.redispool.Get()
	defer conn.Close()
	output, _ := json.Marshal(&value)
	_, err := conn.Do("hset", key, field, string(output))
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (this *AbuRedis) HSetString(key string, field string, value string) error {
	conn := this.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("hset", key, field, value)
	if err != nil {
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (this *AbuRedis) HGet(key string, field string) interface{} {
	conn := this.redispool.Get()
	defer conn.Close()
	ret, err := conn.Do("hget", key, field)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	return ret
}

func (this *AbuRedis) HDel(key string, field string) error {
	conn := this.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("hdel", key, field)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	return nil
}

func (this *AbuRedis) HKeys(key string) []string {
	conn := this.redispool.Get()
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

func (this *AbuRedis) SAdd(key string, value interface{}) error {
	conn := this.redispool.Get()
	defer conn.Close()
	output, _ := json.Marshal(&value)
	_, err := conn.Do("sadd", key, string(output))
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	return nil
}

func (this *AbuRedis) SAddString(key string, value string) error {
	conn := this.redispool.Get()
	defer conn.Close()
	_, err := conn.Do("sadd", key, value)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	return nil
}
func (this *AbuRedis) HSetObject(key string, object map[string]interface{}) error {
	conn := this.redispool.Get()
	defer conn.Close()
	params := []interface{}{}
	params = append(params, key)
	for k, v := range object {
		if v == nil {
			continue
		}
		params = append(params, k)
		typename := reflect.TypeOf(v).Name()
		if strings.Contains(typename, "int") {
			params = append(params, fmt.Sprintf("%d", v))
		} else if strings.Contains(typename, "float") {
			params = append(params, fmt.Sprintf("%f", v))
		} else {
			params = append(params, fmt.Sprint(v))
		}
	}
	_, err := conn.Do("hmset", params...)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	return nil
}

func (this *AbuRedis) HGetAll(key string) *map[string]interface{} {
	conn := this.redispool.Get()
	defer conn.Close()
	ret, err := conn.Do("hgetall", key)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	arrret := ret.([]interface{})
	if len(arrret) == 0 {
		return nil
	}
	mapret := map[string]interface{}{}
	for i := 0; i < len(arrret); i++ {
		if i%2 == 0 {
			mapret[string(arrret[i].([]byte))] = string(arrret[i+1].([]byte))
		}
	}
	return &mapret
}

func (this *AbuRedis) RPush(key string, vals ...interface{}) error {
	conn := this.redispool.Get()
	defer conn.Close()
	data := []interface{}{}
	data = append(data, key)
	data = append(data, vals...)
	_, err := conn.Do("rpush", data...)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	return nil
}

func (this *AbuRedis) LIndex(key string, idx int) interface{} {
	conn := this.redispool.Get()
	defer conn.Close()
	ret, err := conn.Do("lindex", key, idx)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	return ret
}

func (this *AbuRedis) BLPop(key string, timeout int) interface{} {
	conn := this.redispool.Get()
	defer conn.Close()
	ret, err := conn.Do("blpop", key, timeout)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	if ret == nil {
		return nil
	}
	arr := ret.([]interface{})
	return string(arr[1].([]byte))
}

func (this *AbuRedis) SMembers(key string) []interface{} {
	conn := this.redispool.Get()
	defer conn.Close()
	ret, err := conn.Do("smembers", key)
	if err != nil {
		logs.Error(err.Error())
		return []interface{}{}
	}
	if ret == nil {
		return []interface{}{}
	}
	arr := ret.([]interface{})
	return arr
}

func (this *AbuRedis) HMGet(key string, fields ...interface{}) *map[string]interface{} {
	conn := this.redispool.Get()
	defer conn.Close()
	data := []interface{}{}
	data = append(data, key)
	data = append(data, fields...)
	ret, err := conn.Do("hmget", data...)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	arrret := ret.([]interface{})
	if len(arrret) == 0 {
		return nil
	}
	mapret := map[string]interface{}{}
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		if arrret[i] != nil {
			mapret[field.(string)] = string(arrret[i].([]byte))
		}
	}
	return &mapret
}

func (this *AbuRedis) SRem(key string, vals ...interface{}) []interface{} {
	conn := this.redispool.Get()
	defer conn.Close()
	data := []interface{}{}
	data = append(data, key)
	data = append(data, vals...)
	_, err := conn.Do("srem", data...)
	if err != nil {
		logs.Error(err.Error())
		return []interface{}{}
	}
	return nil
}
