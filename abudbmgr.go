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

func (this *AbuDb) Init(prefix string) {
	this.user = GetConfigString(fmt.Sprint(prefix, ".user"), true, "")
	this.password = GetConfigString(fmt.Sprint(prefix, ".password"), true, "")
	this.host = GetConfigString(fmt.Sprint(prefix, ".host"), true, "")
	this.database = GetConfigString(fmt.Sprint(prefix, ".database"), true, "")
	this.port = GetConfigInt(fmt.Sprint(prefix, ".port"), true, 0)
	this.connmaxlifetime = GetConfigInt(fmt.Sprint(prefix, ".connmaxlifetime"), true, 0)
	this.connmaxidletime = GetConfigInt(fmt.Sprint(prefix, ".connmaxidletime"), true, 0)
	this.connmaxidle = GetConfigInt(fmt.Sprint(prefix, ".connmaxidle"), true, 0)
	this.connmaxopen = GetConfigInt(fmt.Sprint(prefix, ".connmaxopen"), true, 0)
	str := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", this.user, this.password, this.host, this.port, this.database)
	db, err := gorm.Open("mysql", str)
	if err != nil {
		logs.Error(err)
		panic(err)
	}
	db.DB().SetMaxIdleConns(this.connmaxidle)
	db.DB().SetMaxOpenConns(this.connmaxopen)
	db.DB().SetConnMaxIdleTime(time.Second * time.Duration(this.connmaxidletime))
	db.DB().SetConnMaxLifetime(time.Second * time.Duration(this.connmaxlifetime))
	this.db = db
	this.logmode = viper.GetBool(fmt.Sprint(prefix, ".logmode"))
	db.LogMode(this.logmode)
	logs.Debug("连接数据库成功:", this.host, this.port, this.database)
}

func (this *AbuDb) conn() *sql.DB {
	return this.db.DB()
}

func (this *AbuDb) Query(query string, args ...any) (*[]map[string]interface{}, error) {
	data, err := this.db.DB().Query(query, args...)
	if err != nil {
		logs.Error(err)
		return nil, err
	}
	return this.GetResult(data), nil
}

func (this *AbuDb) Exec(query string, args ...any) (*sql.Result, error) {
	data, err := this.db.DB().Exec(query, args...)
	if err != nil {
		logs.Error(err)
		return nil, err
	}
	return &data, nil
}

func (this *AbuDb) Table(tablename string) *AbuDbTable {
	dbtable := AbuDbTable{tablename: tablename, selectstr: "*", db: this}
	return &dbtable
}

func (this *AbuDb) CallProcedure(procname string, args ...interface{}) (*map[string]interface{}, error) {
	sql := ""
	for i := 0; i < len(args); i++ {
		sql += "?,"
	}
	if len(sql) > 0 {
		sql = strings.TrimRight(sql, ",")
	}
	sql = fmt.Sprintf("call %s(%s)", procname, sql)

	dbresult, err := this.db.DB().Query(sql, args...)
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

func (this *AbuDb) GetResult(rows *sql.Rows) *[]map[string]interface{} {
	if rows == nil {
		return nil
	}
	data := []map[string]interface{}{}
	for rows.Next() {
		data = append(data, *this.getone(rows))
	}
	rows.Close()
	return &data
}

func (this *AbuDb) getone(rows *sql.Rows) *map[string]interface{} {
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
			} else if typename == "DATETIME" {
				timestr := string(scans[i].([]uint8))
				t, _ := time.ParseInLocation("2006-01-02 15:04:05", timestr, time.Local)
				r := t.UTC().Format("2006-01-02T15:04:05Z")
				data[fields[i]] = r
			} else if typename == "DATE" {
				timestr := string(scans[i].([]uint8))
				timestr += " 00:00:00"
				t, _ := time.ParseInLocation("2006-01-02 15:04:05", timestr, time.Local)
				r := t.UTC().Format("2006-01-02T15:04:05Z")
				data[fields[i]] = r
			} else {
				data[fields[i]] = string(scans[i].([]uint8))
			}
		} else {
			data[fields[i]] = nil
		}
	}
	return &data
}
