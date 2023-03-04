package abugo

import (
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
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

func (c *AbuDb) Table(tablename string) *AbuDbTable {
	dbtable := AbuDbTable{tablename: tablename, selectstr: "*", db: c}
	return &dbtable
}

func (c *AbuDb) CallProcedure(procname string, args ...interface{}) (*map[string]interface{}, error) {
	sql := ""
	for i := 0; i < len(args); i++ {
		sql += "?,"
	}
	if len(sql) > 0 {
		sql = strings.TrimRight(sql, ",")
	}
	sql = fmt.Sprintf("call %s(%s)", procname, sql)

	dbresult, err := c.db.DB().Query(sql, args...)
	if err != nil {
		logs.Error(err)
		return nil, err
	}
	if dbresult.Next() {
		data := make(map[string]interface{})
		fields, _ := dbresult.Columns()
		scans := make([]interface{}, len(fields))
		for i := range scans {
			scans[i] = &scans[i]
		}
		err := dbresult.Scan(scans...)
		if err != nil {
			return nil, err
		}
		ct, _ := dbresult.ColumnTypes()
		for i := range fields {
			if scans[i] != nil {
				typename := ct[i].DatabaseTypeName()
				if typename == "INT" || typename == "BIGINT" || typename == "TINYINT" || typename == "UNSIGNED BIGINT" || typename == "UNSIGNED" {
					if reflect.TypeOf(scans[i]).Name() == "" {
						v, _ := strconv.ParseInt(string(scans[i].([]uint8)), 10, 64)
						data[fields[i]] = v
					} else {
						data[fields[i]] = scans[i]
					}
				} else if typename == "DOUBLE" || typename == "DECIMAL" {
					if reflect.TypeOf(scans[i]).Name() == "" {
						v, _ := strconv.ParseFloat(string(scans[i].([]uint8)), 64)
						data[fields[i]] = v
					} else {
						data[fields[i]] = scans[i]
					}
				} else {
					data[fields[i]] = string(scans[i].([]uint8))
				}
			} else {
				data[fields[i]] = nil
			}
		}
		dbresult.Close()
		return &data, nil
	}
	dbresult.Close()
	return nil, nil
}

func (c *AbuDb) GetResult(rows *sql.Rows) *[]map[string]interface{} {
	if rows == nil {
		return nil
	}
	data := []map[string]interface{}{}
	for rows.Next() {
		data = append(data, *c.getone(rows))
	}
	rows.Close()
	return &data
}

func (c *AbuDb) getone(rows *sql.Rows) *map[string]interface{} {
	data := make(map[string]interface{})
	fields, _ := rows.Columns()
	scans := make([]interface{}, len(fields))
	for i := range scans {
		scans[i] = &scans[i]
	}
	err := rows.Scan(scans...)
	if err != nil {
		logs.Error(err)
		return nil
	}
	ct, _ := rows.ColumnTypes()
	for i := range fields {
		if scans[i] != nil {
			typename := ct[i].DatabaseTypeName()
			if typename == "INT" || typename == "BIGINT" || typename == "TINYINT" || typename == "UNSIGNED BIGINT" || typename == "UNSIGNED" {
				if reflect.TypeOf(scans[i]).Name() == "" {
					v, _ := strconv.ParseInt(string(scans[i].([]uint8)), 10, 64)
					data[fields[i]] = v
				} else {
					data[fields[i]] = scans[i]
				}
			} else if typename == "DOUBLE" || typename == "DECIMAL" {
				if reflect.TypeOf(scans[i]).Name() == "" {
					v, _ := strconv.ParseFloat(string(scans[i].([]uint8)), 64)
					data[fields[i]] = v
				} else {
					data[fields[i]] = scans[i]
				}
			} else {
				data[fields[i]] = string(scans[i].([]uint8))
			}
		} else {
			data[fields[i]] = nil
		}
	}
	return &data
}
