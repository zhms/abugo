package abugo

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

type AbuDbWhere struct {
	Data   map[string]interface{}
	length int
}

func (this *AbuDbWhere) add(opt string, field string, operator string, val interface{}, invalidval interface{}) *AbuDbWhere {
	if this.Data == nil {
		this.Data = make(map[string]interface{})
	}
	if val != invalidval {
		if this.length == 0 {
			opt = ""
		}
		this.Data[fmt.Sprintf("%s %s@%d@%s", opt, field, this.length, operator)] = val
		this.length++
	}
	return this
}

func (this *AbuDbWhere) And(field string, operator string, val interface{}, invalidval interface{}) *AbuDbWhere {
	return this.add("and", field, operator, val, invalidval)
}

func (this *AbuDbWhere) Or(field string, operator string, val interface{}, invalidval interface{}) *AbuDbWhere {
	return this.add("or", field, operator, val, invalidval)
}

func (this *AbuDbWhere) In(field string, operator string, val interface{}, invalidval interface{}) *AbuDbWhere {
	return this.add("in", field, operator, val, invalidval)
}

func (this *AbuDbWhere) Sql() (string, []interface{}) {
	wstr := ""
	wv := []interface{}{}
	type FieldValue struct {
		Sort  int64
		Field string
		Value interface{}
		Opt   string
	}
	order := []FieldValue{}
	for k, v := range this.Data {
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
	return wstr, wv
}

func (this *AbuDbWhere) EndWith(words string) *AbuDbWhere {
	if this.Data == nil {
		this.Data = make(map[string]interface{})
	}
	this.Data[words] = 0
	return this
}
