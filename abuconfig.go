package abugo

import (
	"fmt"

	"github.com/beego/beego/logs"
	"github.com/spf13/viper"
)

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
