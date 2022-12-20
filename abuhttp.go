package abugo

import (
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/beego/beego/logs"
	"github.com/gin-gonic/gin"
	val "github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

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
}

type AbuHttpGropu struct {
	http *AbuHttp
	name string
}

type AbuDbError struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (c *AbuHttp) Static(relativePaths string, root string) {
	c.gin.Static(relativePaths, root)
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

func (ctx *AbuHttpContent) RespFile(savename string, filepath string) {
	ctx.gin.Header("Content-Disposition", fmt.Sprintf("attachment;filename=%s.xls", savename))
	ctx.gin.File(filepath)
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

func (ctx *AbuHttpContent) RespProcedureErr(err *map[string]interface{}) bool {
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