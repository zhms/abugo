package abugo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/beego/beego/logs"
	"github.com/gin-gonic/gin"
	val "github.com/go-playground/validator/v10"
	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

/*
	错误码:
		0. 成功
		1. 没有配置token的redis
		2. 请求header未填写x-token值
		3. 未登录或者登录过期了
		4. 参数格式错误,参数必须是json格式
		5. 权限不足
*/

type H map[string]any

var errormap *map[string]int

type AbuHttpContent struct {
	gin       *gin.Context
	TokenData string
	Token     string
	reqdata   string
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

func (this *AbuHttpContent) RequestData(obj interface{}) error {
	json.Unmarshal([]byte(this.reqdata), &obj)
	validator := val.New()
	err := validator.Struct(obj)
	if err != nil {
		this.RespErr(6, err.Error())
		return errors.New("参数校验错误")
	}
	return nil
}

func (this *AbuHttpContent) Query(key string) string {
	return this.gin.Query(key)
}

func (this *AbuHttpContent) GetIp() string {
	return this.gin.ClientIP()
}

func (this *AbuHttpContent) Gin() *gin.Context {
	return this.gin
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

	upgrader             websocket.Upgrader
	idx_conn             sync.Map
	conn_idx             sync.Map
	connect_callback     AbuWsCallback
	close_callback       AbuWsCallback
	msgtype              sync.Map
	msg_callback         sync.Map
	default_msg_callback AbuWsDefaultMsgCallback
	request_log_callback DBLogCallback
}

func (this *AbuHttp) Static(relativePaths string, root string) {
	this.gin.Static(relativePaths, root)
}

func (ctx *AbuHttpContent) Put(key string, value interface{}) {
	if ctx.gin.Keys == nil {
		ctx.gin.Keys = make(map[string]interface{})
	}
	if ctx.gin.Keys["REPONSE_DATA"] == nil {
		ctx.gin.Keys["REPONSE_DATA"] = make(map[string]interface{})
	}
	if len(key) <= 0 || key == "" {
		ctx.gin.Keys["REPONSE_DATA"] = value
		return
	}
	ctx.gin.Keys["REPONSE_DATA"].(map[string]interface{})[key] = value
}

func (ctx *AbuHttpContent) RespOK(objects ...interface{}) {
	resp := new(HttpResponse)
	resp.Code = 0
	resp.Msg = "success"
	if len(objects) > 0 {
		ctx.Put("data", objects[0])
	}
	resp.Data = ctx.gin.Keys["REPONSE_DATA"]
	if resp.Data == nil {
		resp.Data = make(map[string]interface{})
	}
	ctx.gin.JSON(http.StatusOK, resp)
}

func (ctx *AbuHttpContent) RespJson(obj any) {
	ctx.gin.JSON(http.StatusOK, obj)
}

func (ctx *AbuHttpContent) RespErr(data ...interface{}) {
	resp := new(HttpResponse)
	if len(data) == 2 {
		resp.Code = data[0].(int)
		resp.Msg = data[1].(string)
	} else {
		resp.Msg = data[0].(string)
		resp.Code = -1
		if errormap != nil {
			code, ok := (*errormap)[resp.Msg]
			resp.Code = code
			if !ok {
				resp.Code = -1
			}
		}
	}
	resp.Data = ctx.gin.Keys["REPONSE_DATA"]
	ctx.gin.JSON(http.StatusOK, resp)
}

func (ctx *AbuHttpContent) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	return ctx.gin.SaveUploadedFile(file, dst)
}

func (this *AbuHttp) Init(cfgkey string) {
	port := GetConfigInt(cfgkey+".port", true, 0)
	this.gin = gin.New()
	this.gin.Use(abuhttpcors())
	tokenhost := viper.GetString("server.token.host")
	if len(tokenhost) > 0 {
		this.tokenrefix = fmt.Sprint(GetConfigString("server.project", true, ""), ":", GetConfigString("server.module", true, ""), ":token")
		this.token = new(AbuRedis)
		this.tokenlifetime = GetConfigInt("server.token.lifetime", true, 0)
		this.token.Init("server.token")
	}
	go func() {
		bind := fmt.Sprint("0.0.0.0:", port)
		this.gin.Run(bind)
	}()
	this.upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	if this.token != nil {
		go func() {
			for {
				logdata := this.token.BLPop(fmt.Sprintf("%s:%s:requests", Project(), Module()), 100000)
				if this.request_log_callback != nil {
					this.request_log_callback(logdata)
				}
			}
		}()
	}
	logs.Debug("http listen:", port)
}

func (this *AbuHttp) InitWs(url string) {
	this.gin.GET(url, func(gc *gin.Context) {
		ctx := &AbuHttpContent{gc, "", "", ""}
		this.ws(ctx)
	})
}

func (this *AbuHttp) SetErrorMap(errmap *map[string]int) {
	errormap = errmap
}

type DBLogCallback func(interface{})

func (this *AbuHttp) SetLogCallback(cb DBLogCallback) {
	this.request_log_callback = cb
}

func (this *AbuHttp) OnGet(path string, handler AbuHttpHandler) {
	this.OnGetWithAuth(path, handler, "")
}

func (this *AbuHttp) OnGetWithAuth(path string, handler AbuHttpHandler, auth string) {
	this.gin.GET(path, func(gc *gin.Context) {
		defer func() {
			err := recover()
			if err != nil {
				logs.Error(err)
				stack := debug.Stack()
				logs.Error(string(stack))
			}
		}()
		body, _ := ioutil.ReadAll(gc.Request.Body)
		strbody := string(body)
		if len(strbody) == 0 {
			strbody = "{}"
		}
		ctx := &AbuHttpContent{gc, "", "", strbody}
		if this.token == nil {
			ctx.RespErr(1, "未配置token")
			return
		}
		tokenstr := gc.GetHeader("x-token")
		if len(tokenstr) == 0 {
			ctx.RespErr(2, "请在header填写:x-token")
			return
		}
		keystr := fmt.Sprintf("get%v%v", gc.Request.URL.Path, tokenstr)
		reqid := Md5(keystr)
		lockkey := fmt.Sprint("lock:", reqid)
		if !this.token.SetNx(lockkey, 1) {
			return
		}
		this.token.Expire(lockkey, 10)
		defer func() {
			this.token.Del(lockkey)
		}()
		rediskey := fmt.Sprint(this.tokenrefix, ":", tokenstr)
		tokendata := this.token.Get(rediskey)
		if tokendata == nil {
			ctx.RespErr(3, "未登录或登录已过期")
			return
		}
		this.token.Expire(rediskey, this.tokenlifetime)
		ctx.TokenData = string(tokendata.([]uint8))
		ctx.Token = tokenstr
		var iauthdata interface{}
		if this.token != nil {
			jbody := map[string]interface{}{}
			err := json.Unmarshal([]byte(strbody), &jbody)
			if err != nil {
				ctx.RespErr(4, "参数必须是json格式")
				return
			}
			jtoken := map[string]interface{}{}
			json.Unmarshal([]byte(ctx.TokenData), &jtoken)
			iauthdata = jtoken["AuthData"]
			jlog := gin.H{"ReqPath": gc.Request.URL.Path,
				"ReqData": jbody, "Account": jtoken["Account"], "UserId": jtoken["UserId"],
				"SellerId": jtoken["SellerId"], "ChannelId": jtoken["ChannelId"], "Ip": ctx.GetIp(), "Token": tokenstr}
			strlog, _ := json.Marshal(&jlog)
			this.token.RPush(fmt.Sprintf("%s:%s:requests", Project(), Module()), string(strlog))
		}
		if len(auth) > 0 {
			spauth := strings.Split(auth, ".")
			m := spauth[0]
			s := spauth[1]
			o := spauth[2]
			if len(spauth) == 3 && iauthdata != nil {
				authdata := make(map[string]interface{})
				json.Unmarshal([]byte(iauthdata.(string)), &authdata)
				im, imok := authdata[m]
				if !imok {
					ctx.RespErr(5, "权限不足")
					return
				}
				is, isok := im.(map[string]interface{})[s]
				if !isok {
					ctx.RespErr(5, "权限不足")
					return
				}
				io, iook := is.(map[string]interface{})[o]
				if !iook {
					ctx.RespErr(5, "权限不足")
					return
				}
				if strings.Index(reflect.TypeOf(io).Name(), "float64") < 0 {
					ctx.RespErr(5, "权限不足")
					return
				}
				if InterfaceToInt(io) != 1 {
					ctx.RespErr(5, "权限不足")
					return
				}
			}
		}
		handler(ctx)
	})
}

func (this *AbuHttp) OnGetNoAuth(path string, handler AbuHttpHandler) {
	this.gin.GET(path, func(gc *gin.Context) {
		defer func() {
			err := recover()
			if err != nil {
				logs.Error(err)
				stack := debug.Stack()
				logs.Error(string(stack))
			}
		}()
		body, _ := ioutil.ReadAll(gc.Request.Body)
		strbody := string(body)
		if len(strbody) == 0 {
			strbody = "{}"
		}
		ctx := &AbuHttpContent{gc, "", "", strbody}
		if this.token != nil {
			jbody := map[string]interface{}{}
			err := json.Unmarshal([]byte(strbody), &jbody)
			if err != nil {
				ctx.RespErr(4, "参数必须是json格式")
				return
			}
			jlog := gin.H{"ReqPath": gc.Request.URL.Path, "ReqData": jbody, "Ip": ctx.GetIp()}
			strlog, _ := json.Marshal(&jlog)
			this.token.RPush(fmt.Sprintf("%s:%s:requests", Project(), Module()), string(strlog))
		}
		handler(ctx)
	})
}

func (this *AbuHttp) OnPost(path string, handler AbuHttpHandler) {
	this.OnPostWithAuth(path, handler, "")
}

func (this *AbuHttp) OnPostWithAuth(path string, handler AbuHttpHandler, auth string) {
	this.gin.POST(path, func(gc *gin.Context) {
		defer func() {
			err := recover()
			if err != nil {
				logs.Error(err)
				stack := debug.Stack()
				logs.Error(string(stack))
			}
		}()
		body, _ := ioutil.ReadAll(gc.Request.Body)
		strbody := string(body)
		if len(strbody) == 0 {
			strbody = "{}"
		}
		ctx := &AbuHttpContent{gc, "", "", strbody}
		if this.token == nil {
			ctx.RespErr(1, "未配置token redis")
			return
		}
		tokenstr := gc.GetHeader("x-token")
		if len(tokenstr) == 0 {
			ctx.RespErr(2, "请在header填写:x-token")
			return
		}
		rediskey := fmt.Sprint(this.tokenrefix, ":", tokenstr)
		tokendata := this.token.Get(rediskey)
		if tokendata == nil {
			ctx.RespErr(3, "未登录或登录已过期")
			return
		}
		keystr := fmt.Sprintf("post%v%v", gc.Request.URL.Path, tokenstr)
		reqid := Md5(keystr)
		lockkey := fmt.Sprint("lock:", reqid)
		if !this.token.SetNx(lockkey, 1) {
			return
		}
		this.token.Expire(lockkey, 10)
		defer func() {
			this.token.Del(lockkey)
		}()
		this.token.Expire(rediskey, this.tokenlifetime)
		ctx.TokenData = string(tokendata.([]uint8))
		ctx.Token = tokenstr
		var iauthdata interface{}
		if this.token != nil {
			jbody := map[string]interface{}{}
			err := json.Unmarshal([]byte(strbody), &jbody)
			if err != nil {
				ctx.RespErr(4, "参数必须是json格式")
				return
			}
			jtoken := map[string]interface{}{}
			json.Unmarshal([]byte(ctx.TokenData), &jtoken)
			iauthdata = jtoken["AuthData"]
			jlog := gin.H{"ReqPath": gc.Request.URL.Path,
				"ReqData": jbody, "Account": jtoken["Account"], "UserId": jtoken["UserId"],
				"SellerId": jtoken["SellerId"], "ChannelId": jtoken["ChannelId"], "Ip": ctx.GetIp(), "Token": tokenstr}
			strlog, _ := json.Marshal(&jlog)
			this.token.RPush(fmt.Sprintf("%s:%s:requests", Project(), Module()), string(strlog))
		}
		if len(auth) > 0 {
			spauth := strings.Split(auth, ".")
			m := spauth[0]
			s := spauth[1]
			o := spauth[2]
			if len(spauth) == 3 && iauthdata != nil {
				authdata := make(map[string]interface{})
				json.Unmarshal([]byte(iauthdata.(string)), &authdata)
				im, imok := authdata[m]
				if !imok {
					ctx.RespErr(5, "权限不足")
					return
				}
				is, isok := im.(map[string]interface{})[s]
				if !isok {
					ctx.RespErr(5, "权限不足")
					return
				}
				io, iook := is.(map[string]interface{})[o]
				if !iook {
					ctx.RespErr(5, "权限不足")
					return
				}
				if strings.Index(reflect.TypeOf(io).Name(), "float64") < 0 {
					ctx.RespErr(5, "权限不足")
					return
				}
				if InterfaceToInt(io) != 1 {
					ctx.RespErr(5, "权限不足")
					return
				}
			}
		}
		handler(ctx)
	})
}

func (this *AbuHttp) OnPostNoAuth(path string, handler AbuHttpHandler) {
	this.gin.POST(path, func(gc *gin.Context) {
		defer func() {
			err := recover()
			if err != nil {
				logs.Error(err)
				stack := debug.Stack()
				logs.Error(string(stack))
			}
		}()
		body, _ := ioutil.ReadAll(gc.Request.Body)
		strbody := string(body)
		if len(strbody) == 0 {
			strbody = "{}"
		}
		ctx := &AbuHttpContent{gc, "", "", strbody}
		if this.token != nil {
			jbody := map[string]interface{}{}
			err := json.Unmarshal([]byte(strbody), &jbody)
			if err != nil {
				ctx.RespErr(4, "参数必须是json格式")
				return
			}
			jlog := gin.H{"ReqPath": gc.Request.URL.Path, "ReqData": jbody, "Ip": ctx.GetIp()}
			strlog, _ := json.Marshal(&jlog)
			this.token.RPush(fmt.Sprintf("%s:%s:requests", Project(), Module()), string(strlog))
		}
		handler(ctx)
	})
}

func (this *AbuHttp) SetToken(key string, data interface{}) {
	if this.token == nil {
		return
	}
	this.token.SetEx(fmt.Sprint(this.tokenrefix, ":", key), this.tokenlifetime, data)
}

func (this *AbuHttp) DelToken(key string) {
	if this.token == nil {
		return
	}
	if key == "" {
		return
	}
	this.token.Del(fmt.Sprint(this.tokenrefix, ":", key))
}

func (this *AbuHttp) GetToken(key string) interface{} {
	if this.token == nil {
		return nil
	}
	return this.token.Get(fmt.Sprint(this.tokenrefix, ":", key))
}

func (this *AbuHttp) RenewToken(key string) {
	if this.token == nil {
		return
	}
	this.token.Expire(fmt.Sprint(this.tokenrefix, ":", key), this.tokenlifetime)
}

type AbuWsCallback func(int64)
type AbuWsMsgCallback func(int64, interface{})
type AbuWsDefaultMsgCallback func(int64, string, interface{})
type abumsgdata struct {
	MsgId string      `json:"msgid"`
	Data  interface{} `json:"data"`
}

func (this *AbuHttp) ws(ctx *AbuHttpContent) {
	conn, err := this.upgrader.Upgrade(ctx.Gin().Writer, ctx.Gin().Request, nil)
	if err != nil {
		logs.Error(err)
		return
	}
	defer conn.Close()
	id := AbuId()
	this.idx_conn.Store(id, conn)
	this.conn_idx.Store(conn, id)
	if this.connect_callback != nil {
		this.connect_callback(id)
	}
	for {
		mt, message, err := conn.ReadMessage()
		this.msgtype.Store(id, mt)
		if err != nil {
			break
		}
		md := abumsgdata{}
		err = json.Unmarshal(message, &md)
		if err == nil {
			callback, cbok := this.msg_callback.Load(md.MsgId)
			if cbok {
				go func() {
					defer func() {
						err := recover()
						if err != nil {
							logs.Error(err)
							stack := debug.Stack()
							logs.Error(string(stack))
						}
					}()
					cb := callback.(AbuWsMsgCallback)
					cb(id, md.Data)
				}()
			} else {
				if this.default_msg_callback != nil {
					go func() {
						defer func() {
							err := recover()
							if err != nil {
								logs.Error(err)
								stack := debug.Stack()
								logs.Error(string(stack))
							}
						}()
						this.default_msg_callback(id, md.MsgId, md.Data)
					}()
				}
			}
		}
	}
	_, ccerr := this.idx_conn.Load(id)
	if ccerr {
		this.idx_conn.Delete(id)
		this.conn_idx.Delete(conn)
		if this.close_callback != nil {
			this.close_callback(id)
		}
	}
}

func (this *AbuHttp) WsSendMsg(id int64, msgid string, data interface{}) {
	iconn, connok := this.idx_conn.Load(id)
	if !connok {
		return
	}
	imt, mtok := this.msgtype.Load(id)
	if !mtok {
		imt = 1
		mtok = true
	}
	conn := iconn.(*websocket.Conn)
	mt := imt.(int)
	msg := abumsgdata{msgid, data}
	msgbyte, jerr := json.Marshal(msg)
	if jerr == nil {
		werr := conn.WriteMessage(mt, msgbyte)
		if werr != nil {
			logs.Error(werr)
		}
	}
}

func (this *AbuHttp) WsClose(id int64) {
	iconn, connok := this.idx_conn.Load(id)
	if connok {
		conn := iconn.(*websocket.Conn)
		this.conn_idx.Delete(conn)
		this.idx_conn.Delete(id)
		this.msgtype.Delete(id)
		conn.Close()
	}
}

func (this *AbuHttp) WsAddConnectCallback(callback AbuWsCallback) {
	this.connect_callback = callback
}

func (this *AbuHttp) WsAddMsgCallback(msgid string, callback AbuWsMsgCallback) {
	this.msg_callback.Store(msgid, callback)
}

func (this *AbuHttp) WsDefaultMsgCallback(callback AbuWsDefaultMsgCallback) {
	this.default_msg_callback = callback
}

func (this *AbuHttp) WsAddCloseCallback(callback AbuWsCallback) {
	this.close_callback = callback
}
