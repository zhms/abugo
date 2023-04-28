package abugo

import (
	"database/sql"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/beego/beego/logs"
	_ "github.com/go-sql-driver/mysql"
)

type AbuDbTable struct {
	db        *AbuDb
	dbconn    *sql.DB
	opttype   int
	tablename string
	where     *map[string]interface{}
	selectstr string
	orderby   string
	limit     int
	join      string
	update    *map[string]interface{}
	insert    *map[string]interface{}
	delete    *map[string]interface{}
	pagekey   string
	pageorder string
	dicsql    string
	dicwv     *[]interface{}
	fields    *map[string]int
}

func (this *AbuDbTable) Conn(db *sql.DB) *AbuDbTable {
	this.dbconn = db
	return this
}

func (this *AbuDbTable) TableName(TableName string) *AbuDbTable {
	this.tablename = TableName
	return this
}

func (this *AbuDbTable) Select(SelectStr string) *AbuDbTable {
	this.selectstr = SelectStr
	return this
}

func (this *AbuDbTable) Where(where ...interface{}) *AbuDbTable {
	wheretype := reflect.TypeOf(where[0])
	if strings.Index(wheretype.Name(), "AbuDbWhere") >= 0 {
		t := where[0].(AbuDbWhere)
		this.where = &t.Data
	} else if wheretype.Name() == "string" {
		this.dicsql = where[0].(string)
		for i := 1; i < len(where); i++ {
			*this.dicwv = append(*this.dicwv, where[i])
		}
	} else {
		w := AbuDbWhere{}
		wheretype := reflect.TypeOf(where[0])
		wherevalue := reflect.ValueOf(where[0])
		for i := 0; i < wheretype.NumField(); i++ {
			field := wheretype.Field(i)
			tag, ok := field.Tag.Lookup("sql")
			fn, fnok := field.Tag.Lookup("field")
			fname := field.Name
			if fnok {
				fname = fn
			}
			if ok {
				tags := strings.Split(tag, ",")
				if len(tags) == 2 {
					if field.Type.Name() == "string" {
						w.Add("and", fname, tags[0], wherevalue.Field(i).String(), tags[1])
					} else if field.Type.Name() == "int" {
						iv, _ := strconv.ParseInt(tags[1], 10, 64)
						w.Add("and", fname, tags[0], int(wherevalue.Field(i).Int()), int(iv))
					} else if field.Type.Name() == "int32" {
						iv, _ := strconv.ParseInt(tags[1], 10, 64)
						w.Add("and", fname, tags[0], int32(wherevalue.Field(i).Int()), int32(iv))
					} else if field.Type.Name() == "int64" {
						iv, _ := strconv.ParseInt(tags[1], 10, 64)
						w.Add("and", fname, tags[0], int64(wherevalue.Field(i).Int()), int64(iv))
					} else if field.Type.Name() == "float32" {
						iv, _ := strconv.ParseFloat(tags[1], 64)
						w.Add("and", fname, tags[0], float32(wherevalue.Field(i).Float()), float32(iv))
					} else if field.Type.Name() == "float64" {
						iv, _ := strconv.ParseFloat(tags[1], 64)
						w.Add("and", fname, tags[0], float64(wherevalue.Field(i).Float()), float64(iv))
					}
				} else if len(tags) == 1 {
					if field.Type.Name() == "string" {
						w.Add("and", fname, tags[0], wherevalue.Field(i).String(), nil)
					} else if field.Type.Name() == "int" {
						w.Add("and", fname, tags[0], wherevalue.Field(i).Int(), nil)
					} else if field.Type.Name() == "int32" {
						w.Add("and", fname, tags[0], int32(wherevalue.Field(i).Int()), nil)
					} else if field.Type.Name() == "int64" {
						w.Add("and", fname, tags[0], int64(wherevalue.Field(i).Int()), nil)
					} else if field.Type.Name() == "float32" {
						w.Add("and", fname, tags[0], float32(wherevalue.Field(i).Float()), nil)
					} else if field.Type.Name() == "float64" {
						w.Add("and", fname, tags[0], float64(wherevalue.Field(i).Float()), nil)
					}
				}
			}
		}
		this.where = &w.Data
	}
	return this
}

func (this *AbuDbTable) SetFields(fields *map[string]int) *AbuDbTable {
	this.fields = fields
	return this
}

func (this *AbuDbTable) OrderBy(orderby string) *AbuDbTable {
	this.orderby = orderby
	return this
}

func (this *AbuDbTable) Limit(limit int) *AbuDbTable {
	this.limit = limit
	return this
}

func (this *AbuDbTable) Join(join string) *AbuDbTable {
	this.join = join
	return this
}

func (this *AbuDbTable) GetOne() (*map[string]interface{}, error) {
	sql, wv := this.get_select_sql()
	sql += " limit 1"
	conn := this.dbconn
	if conn == nil {
		conn = this.db.conn()
	}
	if this.db.logmode {
		logs.Debug(sql, wv...)
	}
	dbresult, err := conn.Query(sql, wv...)
	if err != nil {
		logs.Error(sql, wv, err)
		return nil, err
	}
	if dbresult.Next() {
		one := this.getone(dbresult)
		dbresult.Close()
		return one, nil
	}
	dbresult.Close()
	return nil, nil
}

func (this *AbuDbTable) GetList() (*[]map[string]interface{}, error) {
	sql, wv := this.get_select_sql()
	conn := this.dbconn
	if conn == nil {
		conn = this.db.conn()
	}
	if this.db.logmode {
		logs.Debug(sql, wv...)
	}
	dbresult, err := conn.Query(sql, wv...)
	if err != nil {
		logs.Error(sql, wv, err)
		return nil, err
	}
	data := []map[string]interface{}{}
	for dbresult.Next() {
		data = append(data, *this.getone(dbresult))
	}
	dbresult.Close()
	return &data, nil
}

func (this *AbuDbTable) Update(update map[string]interface{}) (int64, error) {
	if this.fields != nil {
		nofields := []string{}
		for k := range update {
			lk := strings.ToLower(k)
			_, ok := (*this.fields)[lk]
			if !ok {
				nofields = append(nofields, k)
			}
		}
		for i := 0; i < len(nofields); i++ {
			delete(update, nofields[i])
		}
	}
	this.update = &update
	sql, wv := this.get_update_sql()
	conn := this.dbconn
	if conn == nil {
		conn = this.db.conn()
	}
	if this.db.logmode {
		logs.Debug(sql, (*wv)...)
	}
	dbresult, err := conn.Exec(sql, (*wv)...)
	if err != nil {
		logs.Error(sql, wv, err)
		return 0, err
	}
	return dbresult.RowsAffected()
}

func (this *AbuDbTable) Insert(insert map[string]interface{}) (int64, error) {
	if this.fields != nil {
		nofields := []string{}
		for k := range insert {
			lk := strings.ToLower(k)
			_, ok := (*this.fields)[lk]
			if !ok {
				nofields = append(nofields, k)
			}
		}
		for i := 0; i < len(nofields); i++ {
			delete(insert, nofields[i])
		}
	}
	this.insert = &insert
	sql, wv := this.get_insert_sql()
	conn := this.dbconn
	if conn == nil {
		conn = this.db.conn()
	}
	if this.db.logmode {
		logs.Debug(sql, (*wv)...)
	}
	dbresult, err := conn.Exec(sql, (*wv)...)
	if err != nil {
		logs.Error(sql, wv, err)
		return 0, err
	}
	return dbresult.LastInsertId()
}

func (this *AbuDbTable) Delete() (int64, error) {
	sql, wv := this.get_delete_sql()
	conn := this.dbconn
	if conn == nil {
		conn = this.db.conn()
	}
	if this.db.logmode {
		logs.Debug(sql, wv...)
	}
	dbresult, err := conn.Exec(sql, wv...)
	if err != nil {
		logs.Error(sql, wv, err)
		return 0, err
	}
	return dbresult.RowsAffected()
}

func (this *AbuDbTable) Replace(insert map[string]interface{}) (int64, error) {
	if this.fields != nil {
		nofields := []string{}
		for k := range insert {
			lk := strings.ToLower(k)
			_, ok := (*this.fields)[lk]
			if !ok {
				nofields = append(nofields, k)
			}
		}
		for i := 0; i < len(nofields); i++ {
			delete(insert, nofields[i])
		}
	}
	this.insert = &insert
	sql, wv := this.get_replace_sql()
	conn := this.dbconn
	if conn == nil {
		conn = this.db.conn()
	}
	if this.db.logmode {
		logs.Debug(sql, wv...)
	}
	dbresult, err := conn.Exec(sql, wv...)
	if err != nil {
		logs.Error(sql, wv, err)
		return 0, err
	}
	return dbresult.LastInsertId()
}

func (this *AbuDbTable) PageData(Page int, PageSize int, orderbyfield string, orderby string) (*[]map[string]interface{}, int64) {
	sql := ""
	wstr := ""
	wv := []interface{}{}
	type FieldValue struct {
		Sort  int64
		Field string
		Value interface{}
		Opt   string
	}
	if this.where == nil {
		this.where = &map[string]interface{}{}
	}
	order := []FieldValue{}
	for k, v := range *this.where {
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
	if len(this.dicsql) > 0 {
		wstr = this.dicsql
		wv = *this.dicwv
	}
	if len(wstr) > 0 {
		sql = fmt.Sprintf("SELECT COUNT(*) AS Total FROM %s where %s", this.tablename, wstr)
	} else {
		sql = fmt.Sprintf("SELECT COUNT(*) AS Total FROM %s %s", this.tablename, wstr)
	}
	if this.db.logmode {
		logs.Debug(sql, wv...)
	}
	dbresult, err := this.db.conn().Query(sql, wv...)
	if err != nil {
		logs.Error(sql, wv, err)
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
		sql = fmt.Sprintf("SELECT %s FROM %s %s WHERE %s  ", this.selectstr, this.tablename, this.join, wstr)
	} else {
		sql = fmt.Sprintf("SELECT %s FROM %s %s ", this.selectstr, this.tablename, this.join)
	}
	orderbyex := "(" + orderbyfield + ") " + orderby
	sql += fmt.Sprintf("order by %s", orderbyex)
	sql += " "
	sql += fmt.Sprintf("limit %d offset %d", PageSize, (Page-1)*PageSize)

	if this.db.logmode {
		logs.Debug(sql, wv...)
	}
	dbresult, err = this.db.conn().Query(sql, wv...)
	if err != nil {
		logs.Error(sql, wv, err)
		return &[]map[string]interface{}{}, 0
	}
	datas := []map[string]interface{}{}
	for dbresult.Next() {
		datas = append(datas, *this.getone(dbresult))
	}
	dbresult.Close()
	return &datas, int64(total)
}

func (this *AbuDbTable) PageDataEx(Page int, PageSize int, orderbyfield string, orderby string) (*[]map[string]interface{}, int64) {
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
	if this.where == nil {
		this.where = &map[string]interface{}{}
	}
	order := []FieldValue{}
	for k, v := range *this.where {
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
	orderby = strings.ToLower(orderby)
	if len(this.dicsql) > 0 {
		wstr = this.dicsql
		wv = *this.dicwv
	}
	if len(wstr) > 0 {
		sql = fmt.Sprintf("SELECT COUNT(%s) AS Total FROM %s where %s", orderbyfield, this.tablename, wstr)
	} else {
		sql = fmt.Sprintf("SELECT COUNT(%s) AS Total FROM %s %s", orderbyfield, this.tablename, wstr)
	}
	if this.db.logmode {
		logs.Debug(sql, wv...)
	}
	dbresult, err := this.db.conn().Query(sql, wv...)
	if err != nil {
		logs.Error(sql, wv, err)
		return &[]map[string]interface{}{}, 0
	}
	dbresult.Next()
	var total int
	dbresult.Scan(&total)
	dbresult.Close()
	if total == 0 {
		return &[]map[string]interface{}{}, 0
	}

	orderbyex := orderbyfield + " " + orderby
	if len(wstr) > 0 {
		sql = fmt.Sprintf("SELECT %s AS MinValue FROM %s where %s order by %s limit %d,1", orderbyfield, this.tablename, wstr, orderbyex, (Page-1)*PageSize)
	} else {
		sql = fmt.Sprintf("SELECT %s AS MinValue FROM %s %s order by %s limit %d,1", orderbyfield, this.tablename, wstr, orderbyex, (Page-1)*PageSize)
	}
	if this.db.logmode {
		logs.Debug(sql, wv...)
	}
	dbresult, err = this.db.conn().Query(sql, wv...)
	if err != nil {
		logs.Error(sql, wv, err)
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
	wstr = fmt.Sprintf("%s %s ? and (%s)", orderbyfield, opt, wstr)
	twv := []interface{}{}
	twv = append(twv, minvalue)
	for i := 0; i < len(wv); i++ {
		twv = append(twv, wv[i])
	}
	wv = twv
	if len(wstr) > 0 {
		sql = fmt.Sprintf("SELECT %s FROM %s %s WHERE %s  ", this.selectstr, this.tablename, this.join, wstr)
	} else {
		sql = fmt.Sprintf("SELECT %s FROM %s %s ", this.selectstr, this.tablename, this.join)
	}
	sql += fmt.Sprintf("order by %s", orderbyex)
	sql += " "

	sql += fmt.Sprintf("limit %d", PageSize)
	if this.db.logmode {
		logs.Debug(sql, wv...)
	}
	dbresult, err = this.db.conn().Query(sql, wv...)
	if err != nil {
		logs.Error(sql, wv, err)
		return &[]map[string]interface{}{}, 0
	}
	datas := []map[string]interface{}{}
	for dbresult.Next() {
		datas = append(datas, *this.getone(dbresult))
	}
	dbresult.Close()
	return &datas, int64(total)
}

func (this *AbuDbTable) getone(rows *sql.Rows) *map[string]interface{} {
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

func (this *AbuDbTable) get_select_sql() (string, []interface{}) {
	sql := ""
	wstr := ""
	wv := []interface{}{}
	type FieldValue struct {
		Sort  int64
		Field string
		Value interface{}
		Opt   string
	}
	if this.where == nil {
		this.where = &map[string]interface{}{}
	}
	order := []FieldValue{}
	for k, v := range *this.where {
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
	if len(this.dicsql) > 0 {
		wstr = this.dicsql
		wv = *this.dicwv
	}
	if len(wstr) > 0 {
		sql = fmt.Sprintf("SELECT %s FROM %s %s WHERE %s ", this.selectstr, this.tablename, this.join, wstr)
	} else {
		sql = fmt.Sprintf("SELECT %s FROM %s %s", this.selectstr, this.tablename, this.join)
	}
	if len(this.orderby) > 0 {
		sql += "order by "
		sql += this.orderby
		sql += " "
	}
	if this.limit > 0 {
		sql += fmt.Sprintf("limit %d ", this.limit)
	}
	return sql, wv
}

func (this *AbuDbTable) get_delete_sql() (string, []interface{}) {
	sql := ""
	wstr := ""
	wv := []interface{}{}
	type FieldValue struct {
		Sort  int64
		Field string
		Value interface{}
		Opt   string
	}
	if this.where == nil {
		this.where = &map[string]interface{}{}
	}
	order := []FieldValue{}
	for k, v := range *this.where {
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
	if len(this.dicsql) > 0 {
		wstr = this.dicsql
		wv = *this.dicwv
	}
	if len(wstr) > 0 {
		sql = fmt.Sprintf("DELETE FROM %s  WHERE %s ", this.tablename, wstr)
	} else {
		sql = fmt.Sprintf("DELETE FROM  %s ", this.tablename)
	}
	return sql, wv
}
func (this *AbuDbTable) get_update_sql() (string, *[]interface{}) {
	sql := ""
	ustr := ""
	uv := []interface{}{}
	for k, v := range *this.update {
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
	if this.where == nil {
		this.where = &map[string]interface{}{}
	}
	order := []FieldValue{}
	for k, v := range *this.where {
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
	if len(this.dicsql) > 0 {
		wstr = this.dicsql
		for i := 0; i < len(*this.dicwv); i++ {
			uv = append(uv, (*this.dicwv)[i])
		}
	}
	if len(wstr) > 0 {
		sql = fmt.Sprintf("UPDATE %s SET%s WHERE %s ", this.tablename, ustr, wstr)
	} else {
		sql = fmt.Sprintf("UPDATE %s SET%s  ", this.tablename, ustr)
	}
	return sql, &uv
}

func (this *AbuDbTable) get_insert_sql() (string, *[]interface{}) {
	sql := ""
	istr := ""
	ivstr := ""
	iv := []interface{}{}
	for k, v := range *this.insert {
		istr += fmt.Sprintf("%s,", k)
		ivstr += "?,"
		iv = append(iv, v)
	}
	if len(this.dicsql) > 0 {
		istr = this.dicsql
		iv = *this.dicwv
	}
	if len(istr) > 0 {
		istr = strings.TrimRight(istr, ",")
		ivstr = strings.TrimRight(ivstr, ",")
	}
	sql = fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", this.tablename, istr, ivstr)
	return sql, &iv
}

func (this *AbuDbTable) get_replace_sql() (string, []interface{}) {
	sql := ""
	istr := ""
	ivstr := ""
	iv := []interface{}{}
	for k, v := range *this.insert {
		istr += fmt.Sprintf("%s,", k)
		ivstr += "?,"
		iv = append(iv, v)
	}
	if len(this.dicsql) > 0 {
		istr = this.dicsql
		iv = *this.dicwv
	}
	if len(istr) > 0 {
		istr = strings.TrimRight(istr, ",")
		ivstr = strings.TrimRight(ivstr, ",")
	}
	sql = fmt.Sprintf("REPLACE INTO %s(%s) VALUES(%s)", this.tablename, istr, ivstr)
	return sql, iv
}
