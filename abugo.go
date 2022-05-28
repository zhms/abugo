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
*/
import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/beego/beego/logs"
	"github.com/garyburd/redigo/redis"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	"github.com/spf13/viper"
)

func get_config_int64(key string,invalval int64) int64{
	val := viper.GetInt64(key)
	if val == invalval{
		err:= fmt.Sprint("read config error:",key)
		logs.Error(err)
		panic(err)
	}
	return val
}

func get_config_int(key string,invalval int) int{
	val:=viper.GetInt(key)
	if val == invalval{
		err:=fmt.Sprint("read config error:",key)
		logs.Error(err)
		panic(err)
	}
	return val
}

func get_config_string(key string,invalval string) string{
	val:=viper.GetString(key)
	if val == invalval{
		err:=fmt.Sprint("read config error:",key)
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

func GetId() int64{
	return idworker.GetId()
}
func GetUuid() string{
	id,_ := uuid.NewV4()
	return id.String()
}
func Run(){
	logs.Debug("*********start*********")
	for i := range abuwsmsgqueue {
		if(i.MsgData == nil){
			i.Ws.dispatch(i.MsgType,i.Id,abumsgdata{},i.callback)
		} else {
			i.Ws.dispatch(i.MsgType,i.Id,*i.MsgData,i.callback)
		}
    }
}
//////////////////////////////////////////////////////////////////////////////////
//abugo初始化
/////////////////////////////////////////////////////////////////////////////////
func Init(){
	gin.SetMode(gin.ReleaseMode)
	logs.EnableFuncCallDepth(true)
	logs.SetLogFuncCallDepth(3)
	logs.SetLogger(logs.AdapterFile, `{"filename":"_log/logfile.log","maxsize":10485760}`)
	logs.SetLogger(logs.AdapterConsole,`{"color":true}`)
	viper.AddConfigPath("./")
	viper.AddConfigPath("./config")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig()
	if err != nil {
		logs.Error(err)
		return
	}
	snowflakenode:= get_config_int64("server.snowflakenode",0)
	if snowflakenode != 0{
		NewIdWorker(snowflakenode)
	}
}

type AbuDbError struct{
	ErrCode int `json:"errcode"`
	ErrMsg string `json:"errmsg"`
}

func GetDbResult(rows* sql.Rows,ref interface{}) * AbuDbError {
	fields,_ := rows.Columns()
	scans := make([]interface{},len(fields))
	for i := range scans {
		scans[i] = &scans[i]
	}
	rows.Scan(scans...)
	data := make(map[string]interface{})
	for i := range fields{
		if scans[i] != nil {
			if reflect.TypeOf(scans[i]).Name() == ""{
				data[fields[i]] = string(scans[i].([]uint8))
			} else{
				data[fields[i]] = scans[i]
			}
		}
	}
	jdata,_ := json.Marshal(&data)
	abuerr := AbuDbError{}
	err := json.Unmarshal(jdata,&abuerr)
	if err != nil{
		return &AbuDbError{1,err.Error()}
	}
	if abuerr.ErrCode != 0 && len(abuerr.ErrMsg) > 0{
		return &abuerr
	}
	err = json.Unmarshal(jdata,ref)
	if err != nil{
		return &AbuDbError{1,err.Error()}
	}
	return nil
}

func GetDbResultNoAbuError(rows* sql.Rows,ref interface{}) error {
	fields,_ := rows.Columns()
	if len(fields) == 0{
		return errors.New("no fields")
	}
	scans := make([]interface{},len(fields))
	for i := range scans {
		scans[i] = &scans[i]
	}
	rows.Scan(scans...)
	data := make(map[string]interface{})
	for i := range fields{
		if reflect.TypeOf(scans[i]).Name() == ""{
			data[fields[i]] = string(scans[i].([]uint8))
		} else{
			data[fields[i]] = scans[i]
		}
	}
	jdata,_ := json.Marshal(&data)
	err := json.Unmarshal(jdata,ref)
	return err
}
//////////////////////////////////////////////////////////////////////////////////
//Http
/////////////////////////////////////////////////////////////////////////////////
const (
	HTTP_SAVE_DATA_KEY = "http_save_api_data_key"
	HTTP_RESPONSE_CODE_OK = 200
	HTTP_RESPONSE_CODE_OK_MESSAGE = "success"
	HTTP_RESPONSE_CODE_ERROR = 100
	HTTP_RESPONSE_CODE_ERROR_MESSAGE = "fail"
	HTTP_RESPONSE_CODE_NOAUTH = 300
	HTTP_RESPONSE_CODE_NOAUTH_MESSAGE = "noauth"
)
type AbuHttpContent struct{
	gin *gin.Context
	TokenData string
	Token string
}

func abuhttpcors() gin.HandlerFunc {
	return func(context *gin.Context) {
		method := context.Request.Method
		context.Header("Access-Control-Allow-Origin", "*")
		context.Header("Access-Control-Allow-Headers", "Content-Type,AccessToken,X-CSRF-Token, Authorization, Token")
		context.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		context.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
		context.Header("Access-Control-Allow-Credentials", "true")
		if method == "OPTIONS" {
			context.AbortWithStatus(http.StatusNoContent)
		}
		context.Next()
	}
}

func (c *AbuHttpContent) RequestData(obj interface{}) error {
	return c.gin.ShouldBindJSON(obj)
}

func (c *AbuHttpContent) Query(key string) string {
	return c.gin.Query(key)
}

func (c *AbuHttpContent) GetIp() string {
	return c.gin.ClientIP()
}

type AbuHttpHandler func(*AbuHttpContent)

type HttpResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data interface{} `json:"data"`
}

type AbuHttp struct {
	gin *gin.Engine
	token *AbuRedis
	tokenrefix string
	tokenlifetime int
}

type AbuHttpGropu struct{
	http* AbuHttp
	name string
}

func (c* AbuHttp) NewGroup(path string) *AbuHttpGropu{
	return &AbuHttpGropu{c,path};
}

func (c* AbuHttpGropu) Get(path string,handlers ...AbuHttpHandler){
	c.http.Get(fmt.Sprint(c.name,path),handlers...)
}

func (c* AbuHttpGropu) GetNoAuth(path string,handlers ...AbuHttpHandler){
	c.http.GetNoAuth(fmt.Sprint(c.name,path),handlers...)
}

func (c* AbuHttpGropu) Post(path string,handlers ...AbuHttpHandler){
	c.http.Post(fmt.Sprint(c.name,path),handlers...)
}

func (c* AbuHttpGropu) PostNoAuth(path string,handlers ...AbuHttpHandler){
	c.http.PostNoAuth(fmt.Sprint(c.name,path),handlers...)
}

func (ctx * AbuHttpContent) Put(key string, value interface{}){
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

func (ctx * AbuHttpContent) RespOK(objects... interface{}) {
	resp := new(HttpResponse)
	resp.Code = HTTP_RESPONSE_CODE_OK
	resp.Msg = HTTP_RESPONSE_CODE_OK_MESSAGE
	if len(objects) > 0 {
		ctx.Put("",objects[0])
	}
	resp.Data = ctx.gin.Keys[HTTP_SAVE_DATA_KEY]
	if resp.Data == nil{
		resp.Data = make(map[string]interface{})
	}
	ctx.gin.JSON(http.StatusOK, resp)
}

func (ctx * AbuHttpContent) RespErr(errcode int,errmsg string) {
	resp := new(HttpResponse)
	ctx.Put("errcode",errcode)
	ctx.Put("errmsg",errmsg)
	resp.Code = HTTP_RESPONSE_CODE_ERROR
	resp.Msg = HTTP_RESPONSE_CODE_ERROR_MESSAGE
	resp.Data = ctx.gin.Keys[HTTP_SAVE_DATA_KEY]
	ctx.gin.JSON(http.StatusOK, resp)
}

func (ctx * AbuHttpContent) RespNoAuth(errcode int,errmsg string) {
	resp := new(HttpResponse)
	ctx.Put("errcode",errcode)
	ctx.Put("errmsg",errmsg)
	resp.Code = HTTP_RESPONSE_CODE_NOAUTH
	resp.Msg = HTTP_RESPONSE_CODE_NOAUTH_MESSAGE
	resp.Data = ctx.gin.Keys[HTTP_SAVE_DATA_KEY]
	ctx.gin.JSON(http.StatusOK, resp)
}

func (c* AbuHttp) Init(cfgkey string){
	port := get_config_int(cfgkey,0)
	c.gin = gin.New()
	c.gin.Use(abuhttpcors())
	tokenhost := viper.GetString("server.token.host")
	if len(tokenhost) > 0{
		c.tokenrefix = get_config_string("server.token.prefix","")
		c.token = new(AbuRedis)
		c.tokenlifetime = get_config_int("server.token.lifetime",0)
		c.token.Init("server.token")
	}
	go func ()  {
		bind:=fmt.Sprint("0.0.0.0:",port)
		c.gin.Run(bind)
	}()
	logs.Debug("http listen:",port)
}

func (c* AbuHttp) Get(path string,handlers ...AbuHttpHandler){
	c.gin.GET(path, func(gc *gin.Context) {
		ctx:= &AbuHttpContent{gc,"",""}
		tokenstr := gc.GetHeader("x-token")
		if len(tokenstr) == 0{
			ctx.RespNoAuth(1,"请在header填写:x-token")
			return
		}
		rediskey := fmt.Sprint(c.tokenrefix,":",tokenstr)
		tokendata := c.token.Get(rediskey)
		if tokendata == nil{
			ctx.RespNoAuth(2,"未登录或登录已过期")
			return
		}
		c.token.Expire(rediskey,c.tokenlifetime)
		ctx.TokenData = string(tokendata.([]uint8))
		ctx.Token = tokenstr
		for i:=range handlers{
			handlers[i](ctx)
		}
	})
}

func (c* AbuHttp) GetNoAuth(path string,handlers ...AbuHttpHandler){
	c.gin.GET(path, func(gc *gin.Context) {
		ctx:= &AbuHttpContent{gc,"",""}
		for i:=range handlers{
			handlers[i](ctx)
		}
	})
}

func (c* AbuHttp) Post(path string,handlers ...AbuHttpHandler){
	c.gin.POST(path, func(gc *gin.Context) {
		ctx:= &AbuHttpContent{gc,"",""}
		tokenstr := gc.GetHeader("x-token")
		if len(tokenstr) == 0{
			ctx.RespNoAuth(1,"请在header填写:x-token")
			return
		}
		rediskey := fmt.Sprint(c.tokenrefix,":",tokenstr)
		tokendata := c.token.Get(rediskey)
		if tokendata == nil{
			ctx.RespNoAuth(2,"未登录或登录已过期")
			return
		}
		c.token.Expire(rediskey,c.tokenlifetime)
		ctx.TokenData = string(tokendata.([]uint8))
		ctx.Token = tokenstr
		for i:=range handlers{
			handlers[i](ctx)
		}
	})
}

func (c* AbuHttp) PostNoAuth(path string,handlers ...AbuHttpHandler){
	c.gin.POST(path, func(gc *gin.Context) {
		ctx:= &AbuHttpContent{gc,"",""}
		for i:=range handlers{
			handlers[i](ctx)
		}
	})
}

func (c* AbuHttp) SetToken(key string,data interface{}){
	if c.token == nil {
		return
	}
	c.token.SetEx(fmt.Sprint(c.tokenrefix,":",key),c.tokenlifetime,data)
}

func (c* AbuHttp) DelToken(key string){
	if c.token == nil {
		return
	}
	c.token.Del(fmt.Sprint(c.tokenrefix,":",key))
}

func (c* AbuHttp) RenewToken(key string){
	if c.token == nil {
		return
	}
	c.token.Expire(fmt.Sprint(c.tokenrefix,":",key),c.tokenlifetime)
}

//////////////////////////////////////////////////////////////////////////////////
//Redis
/////////////////////////////////////////////////////////////////////////////////
type AbuRedisSubCallback func(string)
type AbuRedis struct{
	redispool *redis.Pool
	pubconnection * redis.PubSubConn
	host string
	port int
	db int
	password string
	recving bool
	subscribecallbacks map[string] AbuRedisSubCallback
	mu *sync.RWMutex
}

func (c *AbuRedis) Init(prefix string){
	if c.redispool != nil{
		return
	}
	host := get_config_string(fmt.Sprint(prefix,".host"),"")
	port := get_config_int(fmt.Sprint(prefix,".port"),0)
	db := get_config_int(fmt.Sprint(prefix,".db"),-1)
	password := get_config_string(fmt.Sprint(prefix,".password"),"")
	maxidle := get_config_int(fmt.Sprint(prefix,".maxidle"),0)
	maxactive := get_config_int(fmt.Sprint(prefix,".maxactive"),0)
	idletimeout := get_config_int(fmt.Sprint(prefix,".idletimeout"),0)
	c.redispool = &redis.Pool{
		MaxIdle:     maxidle,
		MaxActive:   maxactive,
		IdleTimeout: time.Duration(idletimeout) * time.Second,
		Wait:        true,
		Dial: func() (redis.Conn, error) {
			con, err := redis.Dial("tcp", fmt.Sprint(host,":",port),
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
	conn, err := redis.Dial("tcp", fmt.Sprint(host,":",port),
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
	c.subscribecallbacks =  make(map[string] AbuRedisSubCallback)
	c.mu = new(sync.RWMutex)
}
func (c *AbuRedis) getcallback(channel  string) AbuRedisSubCallback{
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.subscribecallbacks[channel]
}

func (c *AbuRedis) subscribe(channels ... string){
	c.pubconnection.Subscribe(redis.Args{}.AddFlat(channels)...)
	if !c.recving {
		go func() {
			for{
				imsg := c.pubconnection.Receive()
				msgtype:=reflect.TypeOf(imsg).Name()
				if msgtype == "Message" {
					msg := imsg.(redis.Message)
					callback := c.getcallback(msg.Channel)
					if callback != nil{
						callback(string(msg.Data))
					}
				}
			}
		}()
	}
}

func (c *AbuRedis) Subscribe(channel string,callback AbuRedisSubCallback){
	c.mu.Lock()
	c.subscribecallbacks[channel] = callback
	c.mu.Unlock()
	c.subscribe(channel)
}

func (c *AbuRedis) Publish(k,v interface{}) error{
	conn := c.redispool.Get()
	defer conn.Close()
	output, _ := json.Marshal(&v)
	_,err := conn.Do("publish", k,output)
	if err != nil{
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) Get(k string) interface{}{
	conn := c.redispool.Get()
	defer conn.Close()
	ret, err := conn.Do("get", k)
	if err != nil {
		logs.Error(err.Error())
		return nil
	}
	return ret
}

func (c *AbuRedis) Set(k string,v interface{}) error{
	conn := c.redispool.Get()
	defer conn.Close()
	output, _ := json.Marshal(&v)
	_,err := conn.Do("set", k,output)
	if err != nil{
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) SetEx(k string,to int,v interface{}) error{
	conn := c.redispool.Get()
	defer conn.Close()
	output, _ := json.Marshal(&v)
	_,err := conn.Do("setex", k,to,string(output))
	if err != nil{
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) Del(k string) error{
	conn := c.redispool.Get()
	defer conn.Close()
	_,err := conn.Do("del", k)
	if err != nil{
		logs.Error(err.Error())
		return err
	}
	return nil
}

func (c *AbuRedis) Expire(k string,to int) error{
	conn := c.redispool.Get()
	defer conn.Close()
	_,err := conn.Do("expire", k,to)
	if err != nil{
		logs.Error(err.Error())
		return err
	}
	return nil
}
//////////////////////////////////////////////////////////////////////////////////
//db
/////////////////////////////////////////////////////////////////////////////////
type AbuDb struct{
	user string
	password string
	host string
	port int
	connmaxlifetime int
	database string
	db *sql.DB
	connmaxidletime int
	connmaxidle int
	connmaxopen int
}

func (c *AbuDb) Init(prefix string){
	c.user  = get_config_string(fmt.Sprint(prefix,".user"),"")
	c.password  = get_config_string(fmt.Sprint(prefix,".password"),"")
	c.host  = get_config_string(fmt.Sprint(prefix,".host"),"")
	c.database  = get_config_string(fmt.Sprint(prefix,".database"),"")
	c.port  = get_config_int(fmt.Sprint(prefix,".port"),0)
	c.connmaxlifetime =  get_config_int(fmt.Sprint(prefix,".connmaxlifetime"),0)
	c.connmaxidletime =  get_config_int(fmt.Sprint(prefix,".connmaxidletime"),0)
	c.connmaxidle =  get_config_int(fmt.Sprint(prefix,".connmaxidle"),0)
	c.connmaxopen =  get_config_int(fmt.Sprint(prefix,".connmaxopen"),0)
	str := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",c.user,c.password,c.host,c.port,c.database)
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

func (c *AbuDb) Conn() *sql.DB{
	return c.db
}

//////////////////////////////////////////////////////////////////////////////////
//websocket
/////////////////////////////////////////////////////////////////////////////////

type abumsgqueuestruct struct{
	MsgType int //1链接进入 2链接关闭 3消息
	Id int64
	Ws *AbuWebsocket
	MsgData* abumsgdata
	callback AbuWsCallback
}
var abuwsmsgqueue = make(chan abumsgqueuestruct,10000)

type AbuWsCallback func(int64)
type AbuWsMsgCallback func(int64,string)
type AbuWebsocket struct{
	upgrader websocket.Upgrader
	idx_conn sync.Map
	conn_idx sync.Map
	connect_callback AbuWsCallback
	close_callback AbuWsCallback
	msgtype sync.Map
	msg_callback sync.Map

}
func (c *AbuWebsocket) Init(prefix string){
	port:= get_config_int(fmt.Sprint(prefix,".port"),0)
	c.upgrader = websocket.Upgrader{
		CheckOrigin: func (r * http.Request) bool  {
			return true
		},
	}
	go func() {
		http.HandleFunc("/", c.home)
		bind:=fmt.Sprint("0.0.0.0:",port)
		http.ListenAndServe(bind, nil)
	}()
	logs.Debug("websocket listen:",port)
}
type abumsgdata struct{
	MsgId string `json:"msgid"`
	Data interface{} `json:"data"`
}
func (c *AbuWebsocket) home(w http.ResponseWriter, r *http.Request) {
	conn, err := c.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logs.Error(err)
		return
	}
	defer conn.Close()
	id := GetId()
	c.idx_conn.Store(id,conn)
	c.conn_idx.Store(conn,id)
	{
		mds:= abumsgqueuestruct{1,id,c,nil,nil}
		abuwsmsgqueue <- mds
	}
	for {
		mt, message, err := conn.ReadMessage()
		c.msgtype.Store(id,mt)
		if err != nil {
			break
		}
		md := abumsgdata{}
		err = json.Unmarshal(message,&md)
		if err == nil{
			mds:= abumsgqueuestruct{3,id,c,&md,nil}
			abuwsmsgqueue <- mds
		}
	}
	_,ccerr := c.idx_conn.Load(id)
	if ccerr {
		c.idx_conn.Delete(id)
		c.conn_idx.Delete(conn)
		{
			mds:= abumsgqueuestruct{2,id,c,nil,nil}
			abuwsmsgqueue <- mds
		}
	}
}

func (c *AbuWebsocket) AddConnectCallback(callback AbuWsCallback) {
	c.connect_callback = callback
}

func (c *AbuWebsocket) AddMsgCallback(msgid string,callback AbuWsMsgCallback) {
	c.msg_callback.Store(msgid,callback)
}

func (c *AbuWebsocket) AddCloseCallback(callback AbuWsCallback) {
	c.close_callback = callback
}


func (c *AbuWebsocket) dispatch(msgtype int,id int64, data abumsgdata,ccb AbuWsCallback ) {
	switch msgtype {
	case 3:
		callback,cbok := c.msg_callback.Load(data.MsgId)
		if cbok {
			cb:= callback.(AbuWsMsgCallback)
			jdata,err := json.Marshal(data.Data)
			if err == nil{
				cb(id,string(jdata))
			}
		}
	case 1:
		if c.connect_callback == nil{
			return
		}
		c.connect_callback(id)
	case 2:
		if c.close_callback == nil{
			return
		}
		c.close_callback(id)
	case 4:
		ccb(id)
	}
}

func (c *AbuWebsocket) SendMsg(id int64, msgid string,data interface{} ) {
		iconn,connok := c.idx_conn.Load(id)
		imt,mtok := c.msgtype.Load(id)
		if mtok && connok{
			conn:= iconn.(*websocket.Conn)
			mt:=imt.(int)
			msg := abumsgdata{msgid,data}
			msgbyte,jerr := json.Marshal(msg)
			if jerr == nil {
				werr := conn.WriteMessage(mt,msgbyte)
				if werr != nil {
				}
			}
		}
}

func (c *AbuWebsocket) Close(id int64) {
	iconn,connok := c.idx_conn.Load(id)
	if connok{
		conn:= iconn.(*websocket.Conn)
		c.conn_idx.Delete(conn)
		c.idx_conn.Delete(id)
		c.msgtype.Delete(id)
		conn.Close()
	}
}

func (c *AbuWebsocket) Connect(host string,callback AbuWsCallback) {
	go func ()  {
		conn, _, err := websocket.DefaultDialer.Dial(host, nil)
		if err != nil {
			mds:= abumsgqueuestruct{4,0,c,nil,callback}
			abuwsmsgqueue <- mds
			return
		}
		defer conn.Close()
		id := GetId()
		c.idx_conn.Store(id,conn)
		c.conn_idx.Store(conn,id)
		{
			mds:= abumsgqueuestruct{4,id,c,nil,callback}
			abuwsmsgqueue <- mds
		}
		for {
			mt, message, err := conn.ReadMessage()
			c.msgtype.Store(id,mt)
			if err != nil {
				break
			}
			md := abumsgdata{}
			err = json.Unmarshal(message,&md)
			if err == nil{
				mds:= abumsgqueuestruct{3,id,c,&md,nil}
				abuwsmsgqueue <- mds
			}
		}
		_,ccerr := c.idx_conn.Load(id)
		if ccerr {
			c.idx_conn.Delete(id)
			c.conn_idx.Delete(conn)
			{
				mds:= abumsgqueuestruct{2,id,c,nil,nil}
				abuwsmsgqueue <- mds
			}
		}
	}()
}
