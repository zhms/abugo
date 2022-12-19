package abugo

import (
	"fmt"
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
	fmt.Println("abugo init v1.0.4")
}

func GetConfigInt64(key string, invalval int64) int64 {
	val := viper.GetInt64(key)
	if val == invalval {
		err := fmt.Sprint("read config error:", key)
		logs.Error(err)
		panic(err)
	}
	return val
}

func GetConfigInt(key string, invalval int) int {
	val := viper.GetInt(key)
	if val == invalval {
		err := fmt.Sprint("read config error:", key)
		logs.Error(err)
		panic(err)
	}
	return val
}

func GetConfigString(key string, invalval string) string {
	val := viper.GetString(key)
	if val == invalval {
		err := fmt.Sprint("read config error:", key)
		logs.Error(err)
		panic(err)
	}
	return val
}

func GetConfigFloat(key string, invalval float32) float32 {
	val := viper.GetFloat64(key)
	if val == float64(invalval) {
		err := fmt.Sprint("read config error:", key)
		logs.Error(err)
		panic(err)
	}
	return float32(val)
}

func GetConfigFloat64(key string, invalval float64) float64 {
	val := viper.GetFloat64(key)
	if val == invalval {
		err := fmt.Sprint("read config error:", key)
		logs.Error(err)
		panic(err)
	}
	return val
}
