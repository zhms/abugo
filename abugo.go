package abugo

import (
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
	nodeid := GetConfigInt64("server.node", false, 0)
	if nodeid > 0 {
		NewIdWorker(nodeid)
	}
}
