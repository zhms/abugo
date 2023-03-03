package abugo

import (
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"runtime/debug"
	"sync"

	"github.com/beego/beego/logs"
	"github.com/gin-gonic/gin"
	val "github.com/go-playground/validator/v10"
	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

type H map[string]any

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

func (c *AbuHttpContent) Gin() *gin.Context {
	return c.gin
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
}

func (c *AbuHttp) Static(relativePaths string, root string) {
	c.gin.Static(relativePaths, root)
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

func (ctx *AbuHttpContent) RespJson(obj any) {
	ctx.gin.JSON(http.StatusOK, obj)
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

func (ctx *AbuHttpContent) RespProcErr(err *map[string]interface{}) bool {
	if err != nil && (*err)["errcode"] != nil {
		resp := new(HttpResponse)
		ctx.Put("errcode", InterfaceToInt64((*err)["errcode"]))
		ctx.Put("errmsg", InterfaceToString((*err)["errmsg"]))
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

func (ctx *AbuHttpContent) FromFile(name string) (multipart.File, *multipart.FileHeader, error) {
	return ctx.gin.Request.FormFile(name)
}
func (ctx *AbuHttpContent) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	return ctx.gin.SaveUploadedFile(file, dst)
}

func (c *AbuHttp) Init(cfgkey string) {
	port := GetConfigInt(cfgkey+".port", true, 0)
	c.gin = gin.New()
	c.gin.Use(abuhttpcors())
	tokenhost := viper.GetString("server.token.host")
	if len(tokenhost) > 0 {
		c.tokenrefix = fmt.Sprint(GetConfigString("server.project", true, ""), ":", GetConfigString("server.module", true, ""), ":token")
		c.token = new(AbuRedis)
		c.tokenlifetime = GetConfigInt("server.token.lifetime", true, 0)
		c.token.Init("server.token")
	}
	go func() {
		bind := fmt.Sprint("0.0.0.0:", port)
		c.gin.Run(bind)
	}()
	c.upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	logs.Debug("http listen:", port)
}

func (c *AbuHttp) InitWs(url string) {
	c.gin.GET(url, func(gc *gin.Context) {
		ctx := &AbuHttpContent{gc, "", ""}
		c.ws(ctx)
	})
}

func (c *AbuHttp) Get(path string, handlers ...AbuHttpHandler) {
	c.gin.GET(path, func(gc *gin.Context) {
		defer func() {
			err := recover()
			if err != nil {
				logs.Error(err)
				stack := debug.Stack()
				logs.Error(string(stack))
			}
		}()
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
		defer func() {
			err := recover()
			if err != nil {
				logs.Error(err)
				stack := debug.Stack()
				logs.Error(string(stack))
			}
		}()
		ctx := &AbuHttpContent{gc, "", ""}
		for i := range handlers {
			handlers[i](ctx)
		}
	})
}

func (c *AbuHttp) Post(path string, handlers ...AbuHttpHandler) {
	c.gin.POST(path, func(gc *gin.Context) {
		defer func() {
			err := recover()
			if err != nil {
				logs.Error(err)
				stack := debug.Stack()
				logs.Error(string(stack))
			}
		}()
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
		defer func() {
			err := recover()
			if err != nil {
				logs.Error(err)
				stack := debug.Stack()
				logs.Error(string(stack))
			}
		}()
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
	if key == "" {
		return
	}
	c.token.Del(fmt.Sprint(c.tokenrefix, ":", key))
}

func (c *AbuHttp) GetToken(key string) interface{} {
	if c.token == nil {
		return nil
	}
	return c.token.Get(fmt.Sprint(c.tokenrefix, ":", key))
}

func (c *AbuHttp) RenewToken(key string) {
	if c.token == nil {
		return
	}
	c.token.Expire(fmt.Sprint(c.tokenrefix, ":", key), c.tokenlifetime)
}

type AbuWsCallback func(int64)
type AbuWsMsgCallback func(int64, interface{})
type AbuWsDefaultMsgCallback func(int64, string, interface{})
type abumsgdata struct {
	MsgId string      `json:"msgid"`
	Data  interface{} `json:"data"`
}

func (c *AbuHttp) ws(ctx *AbuHttpContent) {
	conn, err := c.upgrader.Upgrade(ctx.Gin().Writer, ctx.Gin().Request, nil)
	if err != nil {
		logs.Error(err)
		return
	}
	defer conn.Close()
	id := AbuId()
	c.idx_conn.Store(id, conn)
	c.conn_idx.Store(conn, id)
	if c.connect_callback != nil {
		c.connect_callback(id)
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
			callback, cbok := c.msg_callback.Load(md.MsgId)
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
				if c.default_msg_callback != nil {
					go func() {
						defer func() {
							err := recover()
							if err != nil {
								logs.Error(err)
								stack := debug.Stack()
								logs.Error(string(stack))
							}
						}()
						c.default_msg_callback(id, md.MsgId, md.Data)
					}()
				}
			}
		}
	}
	_, ccerr := c.idx_conn.Load(id)
	if ccerr {
		c.idx_conn.Delete(id)
		c.conn_idx.Delete(conn)
		if c.close_callback != nil {
			c.close_callback(id)
		}
	}
}

func (c *AbuHttp) WsSendMsg(id int64, msgid string, data interface{}) {
	iconn, connok := c.idx_conn.Load(id)
	if !connok {
		return
	}
	imt, mtok := c.msgtype.Load(id)
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

func (c *AbuHttp) WsClose(id int64) {
	iconn, connok := c.idx_conn.Load(id)
	if connok {
		conn := iconn.(*websocket.Conn)
		c.conn_idx.Delete(conn)
		c.idx_conn.Delete(id)
		c.msgtype.Delete(id)
		conn.Close()
	}
}

func (c *AbuHttp) WsAddConnectCallback(callback AbuWsCallback) {
	c.connect_callback = callback
}

func (c *AbuHttp) WsAddMsgCallback(msgid string, callback AbuWsMsgCallback) {
	c.msg_callback.Store(msgid, callback)
}

func (c *AbuHttp) WsDefaultMsgCallback(callback AbuWsDefaultMsgCallback) {
	c.default_msg_callback = callback
}

func (c *AbuHttp) WsAddCloseCallback(callback AbuWsCallback) {
	c.close_callback = callback
}
