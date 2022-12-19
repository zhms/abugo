package abugo

import (
	"fmt"
	"strconv"
	"time"

	mrand "math/rand"

	"github.com/beego/beego/logs"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func Init() {
	mrand.Seed(time.Now().Unix())
	gin.SetMode(gin.ReleaseMode)
	logs.EnableFuncCallDepth(true)
	logs.SetLogFuncCallDepth(3)
	logs.SetLogger(logs.AdapterFile, `{"filename":"_log/logfile.log","maxsize":10485760}`)
	logs.SetLogger(logs.AdapterConsole, `{"color":true}`)
	viper.AddConfigPath("./")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig()
	if err != nil {
		logs.Error(err)
		return
	}
	nodeid := GetConfigInt64("server.snowflakeid", false, 0)
	if nodeid > 0 {
		NewIdWorker(nodeid)
	}
}

func InterfaceToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch v.(type) {
	case string:
		return v.(string)
	case int:
		return fmt.Sprint(v.(int))
	case int32:
		return fmt.Sprint(v.(int32))
	case int64:
		return fmt.Sprint(v.(int64))
	case float32:
		return fmt.Sprint(v.(float32))
	case float64:
		return fmt.Sprint(v.(float64))
	}
	return ""
}

func GetMapString(mp *map[string]interface{}, field string) string {
	if mp == nil {
		return ""
	}
	v := (*mp)[field]
	return InterfaceToString(v)
}

func InterfaceToInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch v.(type) {
	case string:
		i, err := strconv.ParseInt(v.(string), 10, 64)
		if err != nil {
			return 0
		}
		return i
	case int:
		return int64(v.(int))
	case int32:
		return int64(v.(int32))
	case int64:
		return int64(v.(int64))
	case float32:
		return int64(v.(float32))
	case float64:
		return int64(v.(float64))
	}
	return 0
}

func GetMapInt64(mp *map[string]interface{}, field string) int64 {
	if mp == nil {
		return 0
	}
	v := (*mp)[field]
	return InterfaceToInt64(v)
}

func InterfaceToInt(v interface{}) int32 {
	if v == nil {
		return 0
	}
	switch v.(type) {
	case string:
		i, err := strconv.ParseInt(v.(string), 10, 64)
		if err != nil {
			return 0
		}
		return int32(i)
	case int:
		return int32(v.(int))
	case int32:
		return int32(v.(int32))
	case int64:
		return int32(v.(int64))
	case float32:
		return int32(v.(float32))
	case float64:
		return int32(v.(float64))
	}
	return 0
}

func GetMapInt(mp *map[string]interface{}, field string) int32 {
	if mp == nil {
		return 0
	}
	v := (*mp)[field]
	return InterfaceToInt(v)
}

func InterfaceToFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch v.(type) {
	case string:
		i, err := strconv.ParseFloat(v.(string), 64)
		if err != nil {
			return 0
		}
		return i
	case int:
		return float64(v.(int))
	case int32:
		return float64(v.(int32))
	case int64:
		return float64(v.(int64))
	case float32:
		return float64(v.(float32))
	case float64:
		return v.(float64)
	}
	return 0
}

func GetMapFloat64(mp *map[string]interface{}, field string) float64 {
	if mp == nil {
		return 0
	}
	v := (*mp)[field]
	return InterfaceToFloat64(v)
}

func InterfaceToFloat(v interface{}) float32 {
	if v == nil {
		return 0
	}
	switch v.(type) {
	case string:
		i, err := strconv.ParseFloat(v.(string), 64)
		if err != nil {
			return 0
		}
		return float32(i)
	case int:
		return float32(v.(int))
	case int32:
		return float32(v.(int32))
	case int64:
		return float32(v.(int64))
	case float32:
		return float32(v.(float32))
	case float64:
		return float32(v.(float64))
	}
	return 0
}

func GetMapFloat(mp *map[string]interface{}, field string) float32 {
	if mp == nil {
		return 0
	}
	v := (*mp)[field]
	return InterfaceToFloat(v)
}
