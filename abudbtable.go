package abugo

import (
	"database/sql"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/beego/beego/logs"
	_ "github.com/go-sql-driver/mysql"
)

type AbuDbTable struct {
	db        *AbuDb
	dbconn    *sql.DB
	opttype   int
	tablename string
	where     map[string]interface{}
	selectstr string
	orderby   string
	limit     int
	join      string
	update    map[string]interface{}
	insert    map[string]interface{}
	pagekey   string
	pageorder string
}

func (c *AbuDbTable) Conn(db *sql.DB) *AbuDbTable {
	c.dbconn = db
	return c
}

func (c *AbuDbTable) TableName(TableName string) *AbuDbTable {
	c.tablename = TableName
	return c
}

func (c *AbuDbTable) Select(SelectStr string) *AbuDbTable {
	c.selectstr = SelectStr
	return c
}

func (c *AbuDbTable) Where(where AbuDbWhere) *AbuDbTable {
	c.where = where.Data
	return c
}

func (c *AbuDbTable) OrderBy(orderby string) *AbuDbTable {
	c.orderby = orderby
	return c
}

func (c *AbuDbTable) Limit(limit int) *AbuDbTable {
	c.limit = limit
	return c
}

func (c *AbuDbTable) Join(join string) *AbuDbTable {
	c.join = join
	return c
}

func (c *AbuDbTable) getone(rows *sql.Rows) *map[string]interface{} {
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
			if typename == "INT" || typename == "BIGINT" || typename == "TINYINT" {
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

func (c *AbuDbTable) get_select_sql() (string, []interface{}) {
	sql := ""
	wstr := ""
	wv := []interface{}{}
	type FieldValue struct {
		Sort  int64
		Field string
		Value interface{}
		Opt   string
	}
	order := []FieldValue{}
	for k, v := range c.where {
		ks := strings.Split(k, "@")
		opt := "="
		if len(ks) == 3 {
			opt = ks[2]
		}
		if len(ks) == 2 || len(ks) == 3 {
			sort, _ := strconv.ParseInt(ks[1], 10, 32)
			order = append(order, FieldValue{Sort: sort, Field: ks[0], Value: v, Opt: opt})
		} else if len(ks) == 1 {
			order = append(order, FieldValue{Sort: 1000000, Field: k, Value: nil, Opt: opt})
		}
	}
	sort.Slice(order, func(i, j int) bool {
		return order[i].Sort < order[j].Sort
	})
	for _, v := range order {
		if v.Value != nil {
			wstr += fmt.Sprintf(" %s %s ? ", v.Field, v.Opt)
			wv = append(wv, v.Value)
		} else {
			wstr += v.Field
		}
	}
	if len(wstr) > 0 {
		sql = fmt.Sprintf("SELECT %s FROM %s %s WHERE %s ", c.selectstr, c.tablename, c.join, wstr)
	} else {
		sql = fmt.Sprintf("SELECT %s FROM %s %s", c.selectstr, c.tablename, c.join)
	}
	if len(c.orderby) > 0 {
		sql += "order by "
		sql += c.orderby
		sql += " "
	}
	if c.limit > 0 {
		sql += fmt.Sprintf("limit %d ", c.limit)
	}
	return sql, wv
}

func (c *AbuDbTable) GetOne() (*map[string]interface{}, error) {
	sql, wv := c.get_select_sql()
	sql += " limit 1"
	conn := c.dbconn
	if conn == nil {
		conn = c.db.Conn()
	}
	if c.db.logmode {
		logs.Debug(sql, wv...)
	}
	dbresult, err := conn.Query(sql, wv...)
	if err != nil {
		logs.Error(sql, wv, err)
		return nil, err
	}
	if dbresult.Next() {
		one := c.getone(dbresult)
		dbresult.Close()
		return one, nil
	}
	dbresult.Close()
	return nil, nil
}

func (c *AbuDbTable) GetList() (*[]map[string]interface{}, error) {
	sql, wv := c.get_select_sql()
	conn := c.dbconn
	if conn == nil {
		conn = c.db.Conn()
	}
	if c.db.logmode {
		logs.Debug(sql, wv...)
	}
	dbresult, err := conn.Query(sql, wv...)
	if err != nil {
		logs.Error(sql, wv, err)
		return nil, err
	}
	data := []map[string]interface{}{}
	for dbresult.Next() {
		data = append(data, *c.getone(dbresult))
	}
	dbresult.Close()
	return &data, nil
}

func (c *AbuDbTable) get_update_sql() (string, []interface{}) {
	sql := ""
	ustr := ""
	uv := []interface{}{}
	for k, v := range c.update {
		ustr += fmt.Sprintf(" %s = ?,", k)
		uv = append(uv, v)
	}
	if len(ustr) > 0 {
		ustr = strings.TrimRight(ustr, ",")
	}
	ustr += " "
	wstr := ""
	type FieldValue struct {
		Sort  int64
		Field string
		Value interface{}
		Opt   string
	}
	order := []FieldValue{}
	for k, v := range c.where {
		ks := strings.Split(k, "@")
		opt := "="
		if len(ks) == 3 {
			opt = ks[2]
		}
		if len(ks) == 2 || len(ks) == 3 {
			sort, _ := strconv.ParseInt(ks[1], 10, 32)
			order = append(order, FieldValue{Sort: sort, Field: ks[0], Value: v, Opt: opt})
		} else if len(ks) == 1 {
			order = append(order, FieldValue{Sort: 1000000, Field: k, Value: nil, Opt: opt})
		}
	}
	sort.Slice(order, func(i, j int) bool {
		return order[i].Sort < order[j].Sort
	})
	for _, v := range order {
		if v.Value != nil {
			wstr += fmt.Sprintf(" %s = ?", v.Field)
			uv = append(uv, v.Value)
		} else {
			wstr += v.Field
		}
	}
	if len(wstr) > 0 {
		sql = fmt.Sprintf("UPDATE %s SET%s WHERE %s ", c.tablename, ustr, wstr)
	} else {
		sql = fmt.Sprintf("UPDATE %s SET%s  ", c.tablename, ustr)
	}
	return sql, uv
}

func (c *AbuDbTable) Update(update map[string]interface{}) (int64, error) {
	c.update = update
	sql, wv := c.get_update_sql()
	conn := c.dbconn
	if conn == nil {
		conn = c.db.Conn()
	}
	if c.db.logmode {
		logs.Debug(sql, wv...)
	}
	dbresult, err := conn.Exec(sql, wv...)
	if err != nil {
		logs.Error(sql, wv, err)
		return 0, err
	}
	return dbresult.RowsAffected()
}

func (c *AbuDbTable) get_insert_sql() (string, []interface{}) {
	sql := ""
	istr := ""
	ivstr := ""
	iv := []interface{}{}
	for k, v := range c.insert {
		istr += fmt.Sprintf("%s,", k)
		ivstr += "?,"
		iv = append(iv, v)
	}
	if len(istr) > 0 {
		istr = strings.TrimRight(istr, ",")
		ivstr = strings.TrimRight(ivstr, ",")
	}
	sql = fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", c.tablename, istr, ivstr)
	return sql, iv
}

func (c *AbuDbTable) Insert(insert map[string]interface{}) (int64, error) {
	c.insert = insert
	sql, wv := c.get_insert_sql()
	conn := c.dbconn
	if conn == nil {
		conn = c.db.Conn()
	}
	if c.db.logmode {
		logs.Debug(sql, wv...)
	}
	dbresult, err := conn.Exec(sql, wv...)
	if err != nil {
		logs.Error(sql, wv, err)
		return 0, err
	}
	return dbresult.LastInsertId()
}

func (c *AbuDbTable) get_replace_sql() (string, []interface{}) {
	sql := ""
	istr := ""
	ivstr := ""
	iv := []interface{}{}
	for k, v := range c.insert {
		istr += fmt.Sprintf("%s,", k)
		ivstr += "?,"
		iv = append(iv, v)
	}
	if len(istr) > 0 {
		istr = strings.TrimRight(istr, ",")
		ivstr = strings.TrimRight(ivstr, ",")
	}
	sql = fmt.Sprintf("REPLACE INTO %s(%s) VALUES(%s)", c.tablename, istr, ivstr)
	return sql, iv
}

func (c *AbuDbTable) Replace(insert map[string]interface{}) (int64, error) {
	c.insert = insert
	sql, wv := c.get_replace_sql()
	conn := c.dbconn
	if conn == nil {
		conn = c.db.Conn()
	}
	if c.db.logmode {
		logs.Debug(sql, wv...)
	}
	dbresult, err := conn.Exec(sql, wv...)
	if err != nil {
		logs.Error(sql, wv, err)
		return 0, err
	}
	return dbresult.LastInsertId()
}

func (c *AbuDbTable) PageData(Page int, PageSize int) (*[]map[string]interface{}, int64) {
	if Page <= 0 {
		Page = 1
	}
	if PageSize <= 0 {
		PageSize = 20
	}
	sql := ""
	wstr := ""
	wv := []interface{}{}
	type FieldValue struct {
		Sort  int64
		Field string
		Value interface{}
		Opt   string
	}
	order := []FieldValue{}
	for k, v := range c.where {
		ks := strings.Split(k, "@")
		opt := "="
		if len(ks) == 3 {
			opt = ks[2]
		}
		if len(ks) == 2 || len(ks) == 3 {
			sort, _ := strconv.ParseInt(ks[1], 10, 32)
			order = append(order, FieldValue{Sort: sort, Field: ks[0], Value: v, Opt: opt})
		} else if len(ks) == 1 {
			order = append(order, FieldValue{Sort: 1000000, Field: k, Value: nil, Opt: opt})
		}
	}
	sort.Slice(order, func(i, j int) bool {
		return order[i].Sort < order[j].Sort
	})
	for _, v := range order {
		if v.Value != nil {
			wstr += fmt.Sprintf(" %s %s ? ", v.Field, v.Opt)
			wv = append(wv, v.Value)
		} else {
			wstr += v.Field
		}
	}
	c.orderby = strings.Trim(c.orderby, " ")
	orderbysplit := strings.Split(c.orderby, " ")
	orderfield := strings.Trim(orderbysplit[0], " ")
	orderby := strings.Trim(orderbysplit[len(orderbysplit)-1], " ")
	orderby = strings.ToLower(orderby)
	if len(wstr) > 0 {
		sql = fmt.Sprintf("SELECT COUNT(%s) AS Total FROM %s where %s", orderfield, c.tablename, wstr)
	} else {
		sql = fmt.Sprintf("SELECT COUNT(%s) AS Total FROM %s %s", orderfield, c.tablename, wstr)
	}
	if c.db.logmode {
		logs.Debug(sql, wv...)
	}
	dbresult, err := c.db.Conn().Query(sql, wv...)
	if err != nil {
		logs.Error(err)
		return &[]map[string]interface{}{}, 0
	}
	dbresult.Next()
	var total int
	dbresult.Scan(&total)
	dbresult.Close()
	if total == 0 {
		return &[]map[string]interface{}{}, 0
	}
	if len(wstr) > 0 {
		sql = fmt.Sprintf("SELECT %s AS MinValue FROM %s where %s order by %s limit %d,1", orderfield, c.tablename, wstr, c.orderby, (Page-1)*PageSize)
	} else {
		sql = fmt.Sprintf("SELECT %s AS MinValue FROM %s %s order by %s limit %d,1", orderfield, c.tablename, wstr, c.orderby, (Page-1)*PageSize)
	}
	if c.db.logmode {
		logs.Debug(sql, wv...)
	}
	dbresult, err = c.db.Conn().Query(sql, wv...)
	if err != nil {
		logs.Error(err)
		return &[]map[string]interface{}{}, 0
	}
	if !dbresult.Next() {
		return &[]map[string]interface{}{}, int64(total)
	}
	var minvalue int
	dbresult.Scan(&minvalue)
	dbresult.Close()
	opt := ""
	if orderby == "asc" {
		opt = ">="
	}
	if orderby == "desc" {
		opt = "<="
	}
	if c.where == nil {
		c.where = make(map[string]interface{})
	}
	c.where[fmt.Sprintf("%s.%s@-1@%s", c.tablename, orderfield, opt)] = minvalue
	wstr = ""
	wv = []interface{}{}
	order = []FieldValue{}
	for k, v := range c.where {
		ks := strings.Split(k, "@")
		opt := "="
		if len(ks) == 3 {
			opt = ks[2]
		}
		if len(ks) == 2 || len(ks) == 3 {
			sort, _ := strconv.ParseInt(ks[1], 10, 32)
			order = append(order, FieldValue{Sort: sort, Field: ks[0], Value: v, Opt: opt})
		} else if len(ks) == 1 {
			order = append(order, FieldValue{Sort: 1000000, Field: k, Value: nil, Opt: opt})
		}
	}
	sort.Slice(order, func(i, j int) bool {
		return order[i].Sort < order[j].Sort
	})
	for k, v := range order {
		if k == 1 {
			if v.Value != nil {
				wstr += fmt.Sprintf(" and %s %s ? ", v.Field, v.Opt)
				wv = append(wv, v.Value)
			} else {
				wstr += v.Field
			}
		} else {
			if v.Value != nil {
				wstr += fmt.Sprintf(" %s %s ? ", v.Field, v.Opt)
				wv = append(wv, v.Value)
			} else {
				wstr += v.Field
			}
		}
	}
	if len(wstr) > 0 {
		sql = fmt.Sprintf("SELECT %s FROM %s %s WHERE %s  ", c.selectstr, c.tablename, c.join, wstr)
	} else {
		sql = fmt.Sprintf("SELECT %s FROM %s %s ", c.selectstr, c.tablename, c.join)
	}
	if len(c.orderby) > 0 {
		sql += fmt.Sprintf("order by %s", c.orderby)
		sql += " "
	}
	sql += fmt.Sprintf("limit %d", PageSize)
	if c.db.logmode {
		logs.Debug(sql, wv...)
	}
	dbresult, err = c.db.Conn().Query(sql, wv...)
	if err != nil {
		logs.Error(err)
		return &[]map[string]interface{}{}, 0
	}
	datas := []map[string]interface{}{}
	for dbresult.Next() {
		datas = append(datas, *c.getone(dbresult))
	}
	dbresult.Close()
	return &datas, int64(total)
}
