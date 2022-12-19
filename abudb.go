package abugo

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/beego/beego/logs"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/spf13/viper"
)

type AbuDb struct {
	user            string
	password        string
	host            string
	port            int
	connmaxlifetime int
	database        string
	db              *gorm.DB
	connmaxidletime int
	connmaxidle     int
	connmaxopen     int
	logmode         bool
}

func (c *AbuDb) Init(prefix string) {
	c.user = GetConfigString(fmt.Sprint(prefix, ".user"), true, "")
	c.password = GetConfigString(fmt.Sprint(prefix, ".password"), true, "")
	c.host = GetConfigString(fmt.Sprint(prefix, ".host"), true, "")
	c.database = GetConfigString(fmt.Sprint(prefix, ".database"), true, "")
	c.port = GetConfigInt(fmt.Sprint(prefix, ".port"), true, 0)
	c.connmaxlifetime = GetConfigInt(fmt.Sprint(prefix, ".connmaxlifetime"), true, 0)
	c.connmaxidletime = GetConfigInt(fmt.Sprint(prefix, ".connmaxidletime"), true, 0)
	c.connmaxidle = GetConfigInt(fmt.Sprint(prefix, ".connmaxidle"), true, 0)
	c.connmaxopen = GetConfigInt(fmt.Sprint(prefix, ".connmaxopen"), true, 0)
	str := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", c.user, c.password, c.host, c.port, c.database)
	db, err := gorm.Open("mysql", str)
	if err != nil {
		logs.Error(err)
		panic(err)
	}
	db.DB().SetMaxIdleConns(c.connmaxidle)
	db.DB().SetMaxOpenConns(c.connmaxopen)
	db.DB().SetConnMaxIdleTime(time.Second * time.Duration(c.connmaxidletime))
	db.DB().SetConnMaxLifetime(time.Second * time.Duration(c.connmaxlifetime))
	c.db = db
	c.logmode = viper.GetBool(fmt.Sprint(prefix, ".logmode"))
	db.LogMode(c.logmode)
	logs.Debug("连接数据库成功:", c.host, c.port, c.database)
}

func (c *AbuDb) Conn() *sql.DB {
	return c.db.DB()
}

func (c *AbuDb) Gorm() *gorm.DB {
	return c.db
}

func (c *AbuDb) Table(tablename string) *AbuDbTable {
	dbtable := AbuDbTable{tablename: tablename, selectstr: "*", db: c}
	return &dbtable
}
