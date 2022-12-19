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

func (c *AbuDbWhere) Add(opt string, field string, operator string, val interface{}, invalidval interface{}) *AbuDbWhere {
	if c.Data == nil {
		c.Data = make(map[string]interface{})
	}
	if val != invalidval {
		if c.length == 0 {
			opt = ""
		}
		c.Data[fmt.Sprintf("%s %s@%d@%s", opt, field, c.length, operator)] = val
		c.length++
	}
	return c
}

func (c *AbuDbWhere) Sql() (string, []interface{}) {
	wstr := ""
	wv := []interface{}{}
	type FieldValue struct {
		Sort  int64
		Field string
		Value interface{}
		Opt   string
	}
	order := []FieldValue{}
	for k, v := range c.Data {
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

func (c *AbuDbWhere) End(field string) *AbuDbWhere {
	if c.Data == nil {
		c.Data = make(map[string]interface{})
	}
	c.Data[field] = 0
	return c
}
