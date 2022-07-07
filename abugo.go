package abugo

/*
	go get github.com/beego/beego/logs
	go get github.com/spf13/viper
	go get github.com/gin-gonic/gin
	go get github.com/go-redis/redis
	go get github.com/garyburd/redigo/redis
	go get github.com/go-sql-driver/mysql
	go get github.com/satori/go.uuid
	go get github.com/gorilla/websocket
	go get github.com/jinzhu/gorm
	go get github.com/imroc/req
	go get github.com/go-playground/validator/v10
	go get github.com/go-playground/universal-translator
*/
import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"database/sql"
	"encoding/asn1"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	mrand "math/rand"

	"github.com/beego/beego/logs"
	"github.com/garyburd/redigo/redis"
	"github.com/gin-gonic/gin"
	val "github.com/go-playground/validator/v10"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	"github.com/spf13/viper"
)

func get_config_int64(key string, invalval int64) int64 {
	val := viper.GetInt64(key)
	if val == invalval {
		err := fmt.Sprint("read config error:", key)
		logs.Error(err)
		panic(err)
	}
	return val
}

func get_config_int(key string, invalval int) int {
	val := viper.GetInt(key)
	if val == invalval {
		err := fmt.Sprint("read config error:", key)
		logs.Error(err)
		panic(err)
	}
	return val
}

func get_config_string(key string, invalval string) string {
	val := viper.GetString(key)
	if val == invalval {
		err := fmt.Sprint("read config error:", key)
		logs.Error(err)
		panic(err)
	}
	return val
}

//////////////////////////////////////////////////////////////////////////////////
//分布式id生成
/////////////////////////////////////////////////////////////////////////////////
const (
	snow_nodeBits  uint8 = 10
	snow_stepBits  uint8 = 12
	snow_nodeMax   int64 = -1 ^ (-1 << snow_nodeBits)
	snow_stepMax   int64 = -1 ^ (-1 << snow_stepBits)
	snow_timeShift uint8 = snow_nodeBits + snow_stepBits
	snow_nodeShift uint8 = snow_stepBits
)

var snow_epoch int64 = 1514764800000

type snowflake struct {
	mu        sync.Mutex
	timestamp int64
	node      int64
	step      int64
}

func (n *snowflake) GetId() int64 {
	n.mu.Lock()
	defer n.mu.Unlock()
	now := time.Now().UnixNano() / 1e6
	if n.timestamp == now {
		n.step++
		if n.step > snow_stepMax {
			for now <= n.timestamp {
				now = time.Now().UnixNano() / 1e6
			}
		}
	} else {
		n.step = 0
	}
	n.timestamp = now
	result := (now-snow_epoch)<<snow_timeShift | (n.node << snow_nodeShift) | (n.step)
	return result
}

type IdWorker interface {
	GetId() int64
}

var idworker IdWorker

func NewIdWorker(node int64) {
	if node < 0 || node > snow_nodeMax {
		panic(fmt.Sprintf("snowflake节点必须在0-%d之间", node))
	}
	snowflakeIns := &snowflake{
		timestamp: 0,
		node:      node,
		step:      0,
	}
	idworker = snowflakeIns
}

func GetId() int64 {
	return idworker.GetId()
}
func GetUuid() string {
	id, _ := uuid.NewV4()
	return id.String()
}
func Run() {
	for i := range abuwsmsgqueue {
		if i.MsgData == nil {
			i.Ws.dispatch(i.MsgType, i.Id, abumsgdata{}, i.callback)
		} else {
			i.Ws.dispatch(i.MsgType, i.Id, *i.MsgData, i.callback)
		}
	}
}

//////////////////////////////////////////////////////////////////////////////////
//abugo初始化
/////////////////////////////////////////////////////////////////////////////////
func Init() {
	mrand.Seed(time.Now().Unix())
	gin.SetMode(gin.ReleaseMode)
	logs.EnableFuncCallDepth(true)
	logs.SetLogFuncCallDepth(3)
	logs.SetLogger(logs.AdapterFile, `{"filename":"_log/logfile.log","maxsize":10485760}`)
	logs.SetLogger(logs.AdapterConsole, `{"color":true}`)
	viper.AddConfigPath("./")
	viper.AddConfigPath("./config")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig()
	if err != nil {
		logs.Error(err)
		return
	}
	snowflakenode := get_config_int64("server.snowflakenode", 0)
	if snowflakenode != 0 {
		NewIdWorker(snowflakenode)
	}
}

type AbuDbError struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func GetDbResult(rows *sql.Rows, ref interface{}) *AbuDbError {
	fields, _ := rows.Columns()
	scans := make([]interface{}, len(fields))
	for i := range scans {
		scans[i] = &scans[i]
	}
	errscan := rows.Scan(scans...)
	if errscan != nil {
		return &AbuDbError{1, errscan.Error()}
	}
	data := make(map[string]interface{})
	ct, _ := rows.ColumnTypes()
	for i := range fields {
		if scans[i] != nil {
			typename := ct[i].DatabaseTypeName()
			if typename == "INT" || typename == "BIGINT" || typename == "TINYINT" {
				if reflect.TypeOf(scans[i]).Name() == "" {
					v, _ := strconv.ParseInt(string(scans[i].([]uint8)), 10, 64)
					data[fields[i]] = v
				} else {
					data[fields[i]] = scans[i]
				}
			} else if typename == "DOUBLE" || typename == "DECIMAL" {
				if reflect.TypeOf(scans[i]).Name() == "" {
					v, _ := strconv.ParseFloat(string(scans[i].([]uint8)), 64)
					data[fields[i]] = v
				} else {
					data[fields[i]] = scans[i]
				}
			} else {
				data[fields[i]] = string(scans[i].([]uint8))
			}
		}
	}
	jdata, _ := json.Marshal(&data)
	abuerr := AbuDbError{}
	err := json.Unmarshal(jdata, &abuerr)
	if err != nil {
		logs.Error(err)
		return &AbuDbError{2, err.Error()}
	}
	if abuerr.ErrCode != 0 && len(abuerr.ErrMsg) > 0 {
		return &abuerr
	}
	err = json.Unmarshal(jdata, ref)
	if err != nil {
		logs.Error(err)
		return &AbuDbError{3, err.Error()}
	}
	return nil
}

func GetDbResultNoAbuError(rows *sql.Rows, ref interface{}) error {
	fields, _ := rows.Columns()
	if len(fields) == 0 {
		return errors.New("no fields")
	}
	scans := make([]interface{}, len(fields))
	for i := range scans {
		scans[i] = &scans[i]
	}
	rows.Scan(scans...)
	data := make(map[string]interface{})
	for i := range fields {
		if reflect.TypeOf(scans[i]).Name() == "" {
			data[fields[i]] = string(scans[i].([]uint8))
		} else {
			data[fields[i]] = scans[i]
		}
	}
	jdata, _ := json.Marshal(&data)
	err := json.Unmarshal(jdata, ref)
	return err
}

//////////////////////////////////////////////////////////////////////////////////
//Http
/////////////////////////////////////////////////////////////////////////////////
const (
	HTTP_SAVE_DATA_KEY                = "http_save_api_data_key"
	HTTP_RESPONSE_CODE_OK             = 200
	HTTP_RESPONSE_CODE_OK_MESSAGE     = "success"
	HTTP_RESPONSE_CODE_ERROR          = 100
	HTTP_RESPONSE_CODE_ERROR_MESSAGE  = "fail"
	HTTP_RESPONSE_CODE_NOAUTH         = 300
	HTTP_RESPONSE_CODE_NOAUTH_MESSAGE = "noauth"
)

type AbuHttpContent struct {
	gin       *gin.Context
	TokenData string
	Token     string
}

func abuhttpcors() gin.HandlerFunc {
	return func(context *gin.Context) {
		method := context.Request.Method
		context.Header("Access-Control-Allow-Origin", "*")
		context.Header("Access-Control-Allow-Headers", "Content-Type, x-token, Content-Length, X-Requested-With")
		context.Header("Access-Control-Allow-Methods", "GET,POST")
		context.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
		context.Header("Access-Control-Allow-Credentials", "true")
		if method == "OPTIONS" {
			context.AbortWithStatus(http.StatusNoContent)
		}
		context.Next()
	}
}

func (c *AbuHttpContent) RequestData(obj interface{}) error {
	c.gin.ShouldBindJSON(obj)
	validator := val.New()
	err := validator.Struct(obj)
	return err
}

func (c *AbuHttpContent) Query(key string) string {
	return c.gin.Query(key)
}

func (c *AbuHttpContent) GetIp() string {
	return c.gin.ClientIP()
}

type AbuHttpHandler func(*AbuHttpContent)

type HttpResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

type AbuHttp struct {
	gin           *gin.Engine
	token         *AbuRedis
	tokenrefix    string
	tokenlifetime int
}

type AbuHttpGropu struct {
	http *AbuHttp
	name string
}

func (c *AbuHttp) NewGroup(path string) *AbuHttpGropu {
	return &AbuHttpGropu{c, path}
}

func (c *AbuHttpGropu) Get(path string, handlers ...AbuHttpHandler) {
	c.http.Get(fmt.Sprint(c.name, path), handlers...)
}

func (c *AbuHttpGropu) GetNoAuth(path string, handlers ...AbuHttpHandler) {
	c.http.GetNoAuth(fmt.Sprint(c.name, path), handlers...)
}

func (c *AbuHttpGropu) Post(path string, handlers ...AbuHttpHandler) {
	c.http.Post(fmt.Sprint(c.name, path), handlers...)
}

func (c *AbuHttpGropu) PostNoAuth(path string, handlers ...AbuHttpHandler) {
	c.http.PostNoAuth(fmt.Sprint(c.name, path), handlers...)
}

func (ctx *AbuHttpContent) Put(key string, value interface{}) {
	if ctx.gin.Keys == nil {
		ctx.gin.Keys = make(map[string]interface{})
	}
	if ctx.gin.Keys[HTTP_SAVE_DATA_KEY] == nil {
		ctx.gin.Keys[HTTP_SAVE_DATA_KEY] = make(map[string]interface{})
	}
	if len(key) <= 0 || key == "" {
		ctx.gin.Keys[HTTP_SAVE_DATA_KEY] = value
		return
	}
	ctx.gin.Keys[HTTP_SAVE_DATA_KEY].(map[string]interface{})[key] = value
}

func (ctx *AbuHttpContent) RespOK(objects ...interface{}) {
	resp := new(HttpResponse)
	resp.Code = HTTP_RESPONSE_CODE_OK
	resp.Msg = HTTP_RESPONSE_CODE_OK_MESSAGE
	if len(objects) > 0 {
		ctx.Put("", objects[0])
	}
	resp.Data = ctx.gin.Keys[HTTP_SAVE_DATA_KEY]
	if resp.Data == nil {
		resp.Data = make(map[string]interface{})
	}
	ctx.gin.JSON(http.StatusOK, resp)
}

func (ctx *AbuHttpContent) RespErr(err error, errcode *int) bool {
	(*errcode)--
	if err != nil {
		resp := new(HttpResponse)
		ctx.Put("errcode", errcode)
		ctx.Put("errmsg", err.Error())
		resp.Code = HTTP_RESPONSE_CODE_ERROR
		resp.Msg = HTTP_RESPONSE_CODE_ERROR_MESSAGE
		resp.Data = ctx.gin.Keys[HTTP_SAVE_DATA_KEY]
		ctx.gin.JSON(http.StatusOK, resp)
	}
	return err != nil
}

func (ctx *AbuHttpContent) RespDbErr(dberr *AbuDbError) bool {
	if dberr != nil && dberr.ErrCode > 0 && len(dberr.ErrMsg) > 0 {
		resp := new(HttpResponse)
		ctx.Put("errcode", dberr.ErrCode)
		ctx.Put("errmsg", dberr.ErrMsg)
		resp.Code = HTTP_RESPONSE_CODE_ERROR
		resp.Msg = HTTP_RESPONSE_CODE_ERROR_MESSAGE
		resp.Data = ctx.gin.Keys[HTTP_SAVE_DATA_KEY]
		ctx.gin.JSON(http.StatusOK, resp)
		return true
	}
	return false
}

func (ctx *AbuHttpContent) RespErrString(err bool, errcode *int, errmsg string) bool {
	(*errcode)--
	if err {
		resp := new(HttpResponse)
		ctx.Put("errcode", errcode)
		ctx.Put("errmsg", errmsg)
		resp.Code = HTTP_RESPONSE_CODE_ERROR
		resp.Msg = HTTP_RESPONSE_CODE_ERROR_MESSAGE
		resp.Data = ctx.gin.Keys[HTTP_SAVE_DATA_KEY]
		ctx.gin.JSON(http.StatusOK, resp)
	}
	return err
}

func (ctx *AbuHttpContent) RespNoAuth(errcode int, errmsg string) {
	resp := new(HttpResponse)
	ctx.Put("errcode", errcode)
	ctx.Put("errmsg", errmsg)
	resp.Code = HTTP_RESPONSE_CODE_NOAUTH
	resp.Msg = HTTP_RESPONSE_CODE_NOAUTH_MESSAGE
	resp.Data = ctx.gin.Keys[HTTP_SAVE_DATA_KEY]
	ctx.gin.JSON(http.StatusOK, resp)
}

func (c *AbuHttp) Init(cfgkey string) {
	port := get_config_int(cfgkey, 0)
	c.gin = gin.New()
	c.gin.Use(abuhttpcors())
	tokenhost := viper.GetString("server.token.host")
	if len(tokenhost) > 0 {
		c.tokenrefix = fmt.Sprint(get_config_string("server.systemname", ""), ":", get_config_string("server.modulename", ""), ":token")
		c.token = new(AbuRedis)
		c.tokenlifetime = get_config_int("server.token.lifetime", 0)
		c.token.Init("server.token")
	}
	go func() {
		bind := fmt.Sprint("0.0.0.0:", port)
		c.gin.Run(bind)
	}()
	logs.Debug("http listen:", port)
}

func (c *AbuHttp) Get(path string, handlers ...AbuHttpHandler) {
	c.gin.GET(path, func(gc *gin.Context) {
		ctx := &AbuHttpContent{gc, "", ""}
		if c.token == nil {
			ctx.RespNoAuth(-1, "未配置token redis")
			return
		}
		tokenstr := gc.GetHeader("x-token")
		if len(tokenstr) == 0 {
			ctx.RespNoAuth(1, "请在header填写:x-token")
			return
		}
		rediskey := fmt.Sprint(c.tokenrefix, ":", tokenstr)
		tokendata := c.token.Get(rediskey)
		if tokendata == nil {
			ctx.RespNoAuth(2, "未登录或登录已过期")
			return
		}
		c.token.Expire(rediskey, c.tokenlifetime)
		ctx.TokenData = string(tokendata.([]uint8))
		ctx.Token = tokenstr
		for i := range handlers {
			handlers[i](ctx)
		}
	})
}

func (c *AbuHttp) GetNoAuth(path string, handlers ...AbuHttpHandler) {
	c.gin.GET(path, func(gc *gin.Context) {
		ctx := &AbuHttpContent{gc, "", ""}
		for i := range handlers {
			handlers[i](ctx)
		}
	})
}

func (c *AbuHttp) Post(path string, handlers ...AbuHttpHandler) {
	c.gin.POST(path, func(gc *gin.Context) {
		ctx := &AbuHttpContent{gc, "", ""}
		if c.token == nil {
			ctx.RespNoAuth(-1, "未配置token redis")
			return
		}
		tokenstr := gc.GetHeader("x-token")
		if len(tokenstr) == 0 {
			ctx.RespNoAuth(1, "请在header填写:x-token")
			return
		}

		rediskey := fmt.Sprint(c.tokenrefix, ":", tokenstr)
		tokendata := c.token.Get(rediskey)
		if tokendata == nil {
			ctx.RespNoAuth(2, "未登录或登录已过期")
			return
		}
		c.token.Expire(rediskey, c.tokenlifetime)
		ctx.TokenData = string(tokendata.([]uint8))
		ctx.Token = tokenstr
		for i := range handlers {
			handlers[i](ctx)
		}
	})
}

func (c *AbuHttp) PostNoAuth(path string, handlers ...AbuHttpHandler) {
	c.gin.POST(path, func(gc *gin.Context) {
		ctx := &AbuHttpContent{gc, "", ""}
		for i := range handlers {
			handlers[i](ctx)
		}
	})
}

func (c *AbuHttp) SetToken(key string, data interface{}) {
	if c.token == nil {
		return
	}
	c.token.SetEx(fmt.Sprint(c.tokenrefix, ":", key), c.tokenlifetime, data)
}

func (c *AbuHttp) DelToken(key string) {
	if c.token == nil {
		return
	}
	c.token.Del(fmt.Sprint(c.tokenrefix, ":", key))
}

func (c *AbuHttp) RenewToken(key string) {
	if c.token == nil {
		return
	}
	c.token.Expire(fmt.Sprint(c.tokenrefix, ":", key), c.tokenlifetime)
}

//////////////////////////////////////////////////////////////////////////////////
//Redis
/////////////////////////////////////////////////////////////////////////////////
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
	host := get_config_string(fmt.Sprint(prefix, ".host"), "")
	port := get_config_int(fmt.Sprint(prefix, ".port"), 0)
	db := get_config_int(fmt.Sprint(prefix, ".db"), -1)
	password := get_config_string(fmt.Sprint(prefix, ".password"), "")
	maxidle := get_config_int(fmt.Sprint(prefix, ".maxidle"), 0)
	maxactive := get_config_int(fmt.Sprint(prefix, ".maxactive"), 0)
	idletimeout := get_config_int(fmt.Sprint(prefix, ".idletimeout"), 0)
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

//////////////////////////////////////////////////////////////////////////////////
//db
/////////////////////////////////////////////////////////////////////////////////
type AbuDb struct {
	user            string
	password        string
	host            string
	port            int
	connmaxlifetime int
	database        string
	db              *sql.DB
	connmaxidletime int
	connmaxidle     int
	connmaxopen     int
}

func (c *AbuDb) Init(prefix string) {
	c.user = get_config_string(fmt.Sprint(prefix, ".user"), "")
	c.password = get_config_string(fmt.Sprint(prefix, ".password"), "")
	c.host = get_config_string(fmt.Sprint(prefix, ".host"), "")
	c.database = get_config_string(fmt.Sprint(prefix, ".database"), "")
	c.port = get_config_int(fmt.Sprint(prefix, ".port"), 0)
	c.connmaxlifetime = get_config_int(fmt.Sprint(prefix, ".connmaxlifetime"), 0)
	c.connmaxidletime = get_config_int(fmt.Sprint(prefix, ".connmaxidletime"), 0)
	c.connmaxidle = get_config_int(fmt.Sprint(prefix, ".connmaxidle"), 0)
	c.connmaxopen = get_config_int(fmt.Sprint(prefix, ".connmaxopen"), 0)
	str := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", c.user, c.password, c.host, c.port, c.database)
	db, err := sql.Open("mysql", str)
	if err != nil {
		logs.Error(err)
		panic(err)
	}
	db.SetMaxIdleConns(c.connmaxidle)
	db.SetMaxOpenConns(c.connmaxopen)
	db.SetConnMaxIdleTime(time.Second * time.Duration(c.connmaxidletime))
	db.SetConnMaxLifetime(time.Second * time.Duration(c.connmaxlifetime))
	c.db = db
}

func (c *AbuDb) Conn() *sql.DB {
	return c.db
}

func (c *AbuDb) QueryNoResult(sqlstr string, args ...interface{}) error {
	result, err := c.db.Query(sqlstr, args...)
	if err != nil {
		logs.Error(err)
	}
	result.Close()
	return err
}

func (c *AbuDb) QueryScan(sqlstr string, params []interface{}, args ...interface{}) (error, bool) {
	result, err := c.db.Query(sqlstr, params...)
	if err != nil {
		logs.Error(err)
		return err, false
	}
	if !result.Next() {
		return nil, false
	}
	result.Scan(args...)
	result.Close()
	return nil, true
}

//////////////////////////////////////////////////////////////////////////////////
//websocket
/////////////////////////////////////////////////////////////////////////////////

type abumsgqueuestruct struct {
	MsgType  int //1链接进入 2链接关闭 3消息
	Id       int64
	Ws       *AbuWebsocket
	MsgData  *abumsgdata
	callback AbuWsCallback
}

var abuwsmsgqueue = make(chan abumsgqueuestruct, 10000)

type AbuWsCallback func(int64)
type AbuWsMsgCallback func(int64, string)
type AbuWebsocket struct {
	upgrader         websocket.Upgrader
	idx_conn         sync.Map
	conn_idx         sync.Map
	connect_callback AbuWsCallback
	close_callback   AbuWsCallback
	msgtype          sync.Map
	msg_callback     sync.Map
}

func (c *AbuWebsocket) Init(prefix string) {
	port := get_config_int(fmt.Sprint(prefix, ".port"), 0)
	c.upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	go func() {
		http.HandleFunc("/", c.home)
		bind := fmt.Sprint("0.0.0.0:", port)
		http.ListenAndServe(bind, nil)
	}()
	logs.Debug("websocket listen:", port)
}

type abumsgdata struct {
	MsgId string      `json:"msgid"`
	Data  interface{} `json:"data"`
}

func (c *AbuWebsocket) home(w http.ResponseWriter, r *http.Request) {
	conn, err := c.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logs.Error(err)
		return
	}
	defer conn.Close()
	id := GetId()
	c.idx_conn.Store(id, conn)
	c.conn_idx.Store(conn, id)
	{
		mds := abumsgqueuestruct{1, id, c, nil, nil}
		abuwsmsgqueue <- mds
	}
	for {
		mt, message, err := conn.ReadMessage()
		c.msgtype.Store(id, mt)
		if err != nil {
			break
		}
		md := abumsgdata{}
		err = json.Unmarshal(message, &md)
		if err == nil {
			mds := abumsgqueuestruct{3, id, c, &md, nil}
			abuwsmsgqueue <- mds
		}
	}
	_, ccerr := c.idx_conn.Load(id)
	if ccerr {
		c.idx_conn.Delete(id)
		c.conn_idx.Delete(conn)
		{
			mds := abumsgqueuestruct{2, id, c, nil, nil}
			abuwsmsgqueue <- mds
		}
	}
}

func (c *AbuWebsocket) AddConnectCallback(callback AbuWsCallback) {
	c.connect_callback = callback
}

func (c *AbuWebsocket) AddMsgCallback(msgid string, callback AbuWsMsgCallback) {
	c.msg_callback.Store(msgid, callback)
}

func (c *AbuWebsocket) AddCloseCallback(callback AbuWsCallback) {
	c.close_callback = callback
}

func (c *AbuWebsocket) dispatch(msgtype int, id int64, data abumsgdata, ccb AbuWsCallback) {
	switch msgtype {
	case 3:
		callback, cbok := c.msg_callback.Load(data.MsgId)
		if cbok {
			cb := callback.(AbuWsMsgCallback)
			jdata, err := json.Marshal(data.Data)
			if err == nil {
				cb(id, string(jdata))
			}
		}
	case 1:
		if c.connect_callback == nil {
			return
		}
		c.connect_callback(id)
	case 2:
		if c.close_callback == nil {
			return
		}
		c.close_callback(id)
	case 4:
		ccb(id)
	}
}

func (c *AbuWebsocket) SendMsg(id int64, msgid string, data interface{}) {
	iconn, connok := c.idx_conn.Load(id)
	imt, mtok := c.msgtype.Load(id)
	if mtok && connok {
		conn := iconn.(*websocket.Conn)
		mt := imt.(int)
		msg := abumsgdata{msgid, data}
		msgbyte, jerr := json.Marshal(msg)
		if jerr == nil {
			werr := conn.WriteMessage(mt, msgbyte)
			if werr != nil {
			}
		}
	}
}

func (c *AbuWebsocket) Close(id int64) {
	iconn, connok := c.idx_conn.Load(id)
	if connok {
		conn := iconn.(*websocket.Conn)
		c.conn_idx.Delete(conn)
		c.idx_conn.Delete(id)
		c.msgtype.Delete(id)
		conn.Close()
	}
}

func (c *AbuWebsocket) Connect(host string, callback AbuWsCallback) {
	go func() {
		conn, _, err := websocket.DefaultDialer.Dial(host, nil)
		if err != nil {
			mds := abumsgqueuestruct{4, 0, c, nil, callback}
			abuwsmsgqueue <- mds
			return
		}
		defer conn.Close()
		id := GetId()
		c.idx_conn.Store(id, conn)
		c.conn_idx.Store(conn, id)
		{
			mds := abumsgqueuestruct{4, id, c, nil, callback}
			abuwsmsgqueue <- mds
		}
		for {
			mt, message, err := conn.ReadMessage()
			c.msgtype.Store(id, mt)
			if err != nil {
				break
			}
			md := abumsgdata{}
			err = json.Unmarshal(message, &md)
			if err == nil {
				mds := abumsgqueuestruct{3, id, c, &md, nil}
				abuwsmsgqueue <- mds
			}
		}
		_, ccerr := c.idx_conn.Load(id)
		if ccerr {
			c.idx_conn.Delete(id)
			c.conn_idx.Delete(conn)
			{
				mds := abumsgqueuestruct{2, id, c, nil, nil}
				abuwsmsgqueue <- mds
			}
		}
	}()
}

func RsaSign(data interface{}, privatekey string) string {
	privatekey = strings.Replace(privatekey, "-----BEGIN PRIVATE KEY-----", "", -1)
	privatekey = strings.Replace(privatekey, "-----END PRIVATE KEY-----", "", -1)
	privatekey = strings.Replace(privatekey, "-----BEGIN RSA PRIVATE KEY-----", "", -1)
	privatekey = strings.Replace(privatekey, "-----END RSA PRIVATE KEY-----", "", -1)
	t := reflect.TypeOf(data)
	v := reflect.ValueOf(data)
	keys := []string{}
	for i := 0; i < t.NumField(); i++ {
		fn := strings.ToLower(t.Field(i).Name)
		if fn != "sign" {
			keys = append(keys, t.Field(i).Name)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	var sb strings.Builder
	for i := 0; i < len(keys); i++ {
		switch sv := v.FieldByName(keys[i]).Interface().(type) {
		case string:
			sb.WriteString(sv)
		case int:
			sb.WriteString(fmt.Sprint(sv))
		case int8:
			sb.WriteString(fmt.Sprint(sv))
		case int16:
			sb.WriteString(fmt.Sprint(sv))
		case int32:
			sb.WriteString(fmt.Sprint(sv))
		case int64:
			sb.WriteString(fmt.Sprint(sv))
		case float32:
			sb.WriteString(fmt.Sprint(sv))
		case float64:
			sb.WriteString(fmt.Sprint(sv))
		}
	}
	privatekeybase64, errb := base64.StdEncoding.DecodeString(privatekey)
	if errb != nil {
		logs.Error(errb)
		return ""
	}
	privatekeyx509, errc := x509.ParsePKCS8PrivateKey([]byte(privatekeybase64))
	if errc != nil {
		logs.Error(errc)
		return ""
	}
	hashmd5 := md5.Sum([]byte(sb.String()))
	hashed := hashmd5[:]
	sign, errd := rsa.SignPKCS1v15(crand.Reader, privatekeyx509.(*rsa.PrivateKey), crypto.MD5, hashed)
	if errd != nil {
		logs.Error(errd)
		return ""
	}
	return base64.StdEncoding.EncodeToString(sign)
}

func RsaVerify(data interface{}, publickey string) bool {
	publickey = strings.Replace(publickey, "-----BEGIN PUBLIC KEY-----", "", -1)
	publickey = strings.Replace(publickey, "-----END PUBLIC KEY-----", "", -1)
	publickey = strings.Replace(publickey, "-----BEGIN RSA PUBLIC KEY-----", "", -1)
	publickey = strings.Replace(publickey, "-----END RSA PUBLIC KEY-----", "", -1)
	t := reflect.TypeOf(data)
	v := reflect.ValueOf(data)
	keys := []string{}
	for i := 0; i < t.NumField(); i++ {
		fn := strings.ToLower(t.Field(i).Name)
		if fn != "sign" {
			keys = append(keys, t.Field(i).Name)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	var sb strings.Builder
	for i := 0; i < len(keys); i++ {
		switch sv := v.FieldByName(keys[i]).Interface().(type) {
		case string:
			sb.WriteString(sv)
		case int:
			sb.WriteString(fmt.Sprint(sv))
		case int8:
			sb.WriteString(fmt.Sprint(sv))
		case int16:
			sb.WriteString(fmt.Sprint(sv))
		case int32:
			sb.WriteString(fmt.Sprint(sv))
		case int64:
			sb.WriteString(fmt.Sprint(sv))
		case float32:
			sb.WriteString(fmt.Sprint(sv))
		case float64:
			sb.WriteString(fmt.Sprint(sv))
		}
	}
	signedstr := fmt.Sprint(v.FieldByName("Sign"))
	publickeybase64, errb := base64.StdEncoding.DecodeString(publickey)
	if errb != nil {
		logs.Error(errb)
		return false
	}
	publickeyx509, errc := x509.ParsePKIXPublicKey([]byte(publickeybase64))
	if errc != nil {
		logs.Error(errc)
		return false
	}
	hash := md5.New()
	hash.Write([]byte(sb.String()))
	signdata, _ := base64.StdEncoding.DecodeString(signedstr)
	errd := rsa.VerifyPKCS1v15(publickeyx509.(*rsa.PublicKey), crypto.MD5, hash.Sum(nil), signdata)
	return errd == nil
}

func aesPKCS7Padding(ciphertext []byte, blocksize int) []byte {
	padding := blocksize - len(ciphertext)%blocksize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}
func aesPKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func AesEncrypt(orig string, key string) string {
	origData := []byte(orig)
	k := []byte(key)
	block, erra := aes.NewCipher(k)
	if erra != nil {
		logs.Error(erra)
		return ""
	}
	blockSize := block.BlockSize()
	origData = aesPKCS7Padding(origData, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, k[:blockSize])
	cryted := make([]byte, len(origData))
	blockMode.CryptBlocks(cryted, origData)
	return base64.StdEncoding.EncodeToString(cryted)
}

func AesDecrypt(cryted string, key string) string {
	crytedByte, _ := base64.StdEncoding.DecodeString(cryted)
	k := []byte(key)
	block, _ := aes.NewCipher(k)
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, k[:blockSize])
	orig := make([]byte, len(crytedByte))
	blockMode.CryptBlocks(orig, crytedByte)
	orig = aesPKCS7UnPadding(orig)
	return string(orig)
}

type AbuRsaKey struct {
	Public  string
	Private string
}

type abuPKCS8Key struct {
	Version             int
	PrivateKeyAlgorithm []asn1.ObjectIdentifier
	PrivateKey          []byte
}

func abuMarshalPKCS8PrivateKey(key *rsa.PrivateKey) ([]byte, error) {
	var pkey abuPKCS8Key
	pkey.Version = 0
	pkey.PrivateKeyAlgorithm = make([]asn1.ObjectIdentifier, 1)
	pkey.PrivateKeyAlgorithm[0] = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
	pkey.PrivateKey = x509.MarshalPKCS1PrivateKey(key)
	return asn1.Marshal(pkey)
}

func NewRsaKey() *AbuRsaKey {
	key := &AbuRsaKey{}
	privateKey, _ := rsa.GenerateKey(crand.Reader, 2048)
	bytes, _ := abuMarshalPKCS8PrivateKey(privateKey)
	privateblock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: bytes,
	}
	key.Private = string(pem.EncodeToMemory(privateblock))
	PublicKey := &privateKey.PublicKey
	pkixPublicKey, _ := x509.MarshalPKIXPublicKey(PublicKey)
	publicblock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pkixPublicKey,
	}
	key.Public = string(pem.EncodeToMemory(publicblock))
	return key
}

func abuGoogleRandStr(strSize int) string {
	dictionary := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	var bytes = make([]byte, strSize)
	_, _ = crand.Read(bytes)
	for k, v := range bytes {
		bytes[k] = dictionary[v%byte(len(dictionary))]
	}
	return string(bytes)
}

func GetGoogleSecret() string {
	return strings.ToUpper(abuGoogleRandStr(32))
}

func abuOneTimePassword(key []byte, value []byte) uint32 {
	hmacSha1 := hmac.New(sha1.New, key)
	hmacSha1.Write(value)
	hash := hmacSha1.Sum(nil)
	offset := hash[len(hash)-1] & 0x0F
	hashParts := hash[offset : offset+4]
	hashParts[0] = hashParts[0] & 0x7F
	number := abuToUint32(hashParts)
	pwd := number % 1000000
	return pwd
}

func abuToUint32(bytes []byte) uint32 {
	return (uint32(bytes[0]) << 24) + (uint32(bytes[1]) << 16) +
		(uint32(bytes[2]) << 8) + uint32(bytes[3])
}

func abuToBytes(value int64) []byte {
	var result []byte
	mask := int64(0xFF)
	shifts := [8]uint16{56, 48, 40, 32, 24, 16, 8, 0}
	for _, shift := range shifts {
		result = append(result, byte((value>>shift)&mask))
	}
	return result
}

func GetGoogleCode(secret string) int32 {
	key, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		logs.Error(err)
		return 0
	}
	epochSeconds := time.Now().Unix() + 0
	return int32(abuOneTimePassword(key, abuToBytes(epochSeconds/30)))
}

func VerifyGoogleCode(secret string, code string) bool {
	nowcode := GetGoogleCode(secret)
	if fmt.Sprint(nowcode) == code {
		return true
	}
	return false
}

func ReadAllText(path string) string {
	file, err := os.Open(path)
	if err != nil {
		logs.Error(err)
		return ""
	}
	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		logs.Error(err)
		return ""
	}
	return string(bytes)
}

func TimeToUtc(timestr string) string {
	t, _ := time.ParseInLocation("2006-01-02 15:04:05", timestr, time.Local)
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

type AbuWhere struct {
	OrderKey string
	OrderBy  string
	sql      string
	Params   []interface{}
}

func (c *AbuWhere) Clean() {
	c.sql = ""
	c.Params = []interface{}{}
}

func (c *AbuWhere) Append(ch string) {
	c.sql += ch
}

func (c *AbuWhere) AddInt(o string, f string, v int, iv int) {
	if v == iv {
		return
	}
	if len(c.sql) == 0 {
		c.sql += "where "
	}
	if c.sql != "where " {
		c.sql += " "
		c.sql += o
	}
	c.sql += " "
	c.sql += f
	c.sql += " = ?"
	c.Params = append(c.Params, v)
}

func (c *AbuWhere) AddInt32(o string, f string, v int32, iv int32) {
	if v == iv {
		return
	}
	if len(c.sql) == 0 {
		c.sql += "where "
	}
	if c.sql != "where " {
		c.sql += " "
		c.sql += o
	}
	c.sql += " "
	c.sql += f
	c.sql += " = ?"
	c.Params = append(c.Params, v)
}

func (c *AbuWhere) AddInt64(o string, f string, v int64, iv int64) {
	if v == iv {
		return
	}
	if len(c.sql) == 0 {
		c.sql += "where "
	}
	if c.sql != "where " {
		c.sql += " "
		c.sql += o
	}
	c.sql += " "
	c.sql += f
	c.sql += " = ?"
	c.Params = append(c.Params, v)
}

func (c *AbuWhere) AddString(o string, f string, v string, iv string) {
	if v == iv {
		return
	}
	if len(c.sql) == 0 {
		c.sql += "where "
	}
	if c.sql != "where " {
		c.sql += " "
		c.sql += o
	}
	c.sql += " "
	c.sql += f
	c.sql += " = ?"
	c.Params = append(c.Params, v)
}

func (c *AbuWhere) AddFloat32(o string, f string, v float32, iv float32) {
	if v == iv {
		return
	}
	if len(c.sql) == 0 {
		c.sql += "where "
	}
	if c.sql != "where " {
		c.sql += " "
		c.sql += o
	}
	c.sql += " "
	c.sql += f
	c.sql += " = ?"
	c.Params = append(c.Params, v)
}

func (c *AbuWhere) AddFloat64(o string, f string, v float64, iv float64) {
	if v == iv {
		return
	}
	if len(c.sql) == 0 {
		c.sql += "where "
	}
	if c.sql != "where " {
		c.sql += " "
		c.sql += o
	}
	c.sql += " "
	c.sql += f
	c.sql += " = ?"
	c.Params = append(c.Params, v)
}

func (c *AbuWhere) Sql(table string, page int, pagesize int) string {
	if len(c.OrderBy) == 0 {
		c.OrderBy = "DESC"
	}
	if len(c.OrderKey) == 0 {
		c.OrderKey = "Id"
	}
	if strings.ToUpper(c.OrderBy) == "DESC" {
		sql := fmt.Sprintf("SELECT * FROM %s WHERE %s <= (SELECT %s FROM %s %s ORDER BY %s %s LIMIT %d,1) %s ORDER BY %s %s LIMIT %d", table, c.OrderKey, c.OrderKey, table, c.sql, c.OrderKey, c.OrderBy, (page-1)*pagesize, strings.Replace(c.sql, "where", "and", -1), c.OrderKey, c.OrderBy, pagesize)
		return sql
	} else {
		c.OrderBy = "ASC"
		sql := fmt.Sprintf("SELECT * FROM %s WHERE %s >= (SELECT %s FROM %s %s ORDER BY %s %s LIMIT %d,1) %s ORDER BY %s %s LIMIT %d", table, c.OrderKey, c.OrderKey, table, c.sql, c.OrderKey, c.OrderBy, (page-1)*pagesize, strings.Replace(c.sql, "where", "and", -1), c.OrderKey, c.OrderBy, pagesize)
		return sql
	}
}

func (c *AbuWhere) CountSql(table string) string {
	if len(c.OrderKey) == 0 {
		c.OrderKey = "Id"
	}
	sql := fmt.Sprintf("select count(%s) as count from %s %s", c.OrderKey, table, c.sql)
	return sql
}

func (c *AbuWhere) GetParams() []interface{} {
	params := []interface{}{}
	params = append(params, c.Params...)
	params = append(params, c.Params...)
	return params
}
