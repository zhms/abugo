package abugo

import (
	"fmt"

	"github.com/beego/beego/logs"
	"github.com/spf13/viper"
)

func GetConfigInt64(key string, require bool, invalval int64) int64 {
	val := viper.GetInt64(key)
	if require && val == invalval {
		err := fmt.Sprint("read config error:", key)
		logs.Error(err)
		panic(err)
	}
	return val
}

func GetConfigInt(key string, require bool, invalval int) int {
	val := viper.GetInt(key)
	if require && val == invalval {
		err := fmt.Sprint("read config error:", key)
		logs.Error(err)
		panic(err)
	}
	return val
}

func GetConfigString(key string, require bool, invalval string) string {
	val := viper.GetString(key)
	if require && val == invalval {
		err := fmt.Sprint("read config error:", key)
		logs.Error(err)
		panic(err)
	}
	return val
}

func GetConfigFloat(key string, require bool, invalval float32) float32 {
	val := viper.GetFloat64(key)
	if require && val == float64(invalval) {
		err := fmt.Sprint("read config error:", key)
		logs.Error(err)
		panic(err)
	}
	return float32(val)
}

func GetConfigFloat64(key string, require bool, invalval float64) float64 {
	val := viper.GetFloat64(key)
	if require && val == invalval {
		err := fmt.Sprint("read config error:", key)
		logs.Error(err)
		panic(err)
	}
	return val
}
