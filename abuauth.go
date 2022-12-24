package abugo

import (
	"encoding/json"
	"reflect"
	"strings"
)

func AuthInit(db *AbuDb, fullauth string) {
	jdata := map[string]interface{}{}
	json.Unmarshal([]byte(fullauth), &jdata)
	xitong := jdata["系统管理"].(map[string]interface{})
	xitong["运营商管理"] = map[string]interface{}{"查:": 0, "增": 0, "删": 0, "改": 0}
	xitongsezhi := xitong["系统设置"].(map[string]interface{})
	xitongsezhi["删"] = 0
	jbytes, _ := json.Marshal(&jdata)
	authstr := string(jbytes)
	sql := "select SellerId from x_seller"
	sellers, _ := db.Conn().Query(sql)
	for sellers.Next() {
		var SellerId int
		sellers.Scan(&SellerId)
		sql = "select * from abu_admin_role where SellerId = ?"
		roles, _ := db.Conn().Query(sql, SellerId)
		if !roles.Next() {
			sql = "insert into abu_admin_role(RoleName,SellerId,Parent,RoleData)values(?,?,?,?)"
			db.Conn().Exec(sql, "运营商超管", SellerId, "god", authstr)
		}
		roles.Close()
	}
	sellers.Close()
	sql = "update abu_admin_role set RoleData = ? where RoleName = ?"
	db.Conn().Exec(sql, authstr, "运营商超管")

	sql = "select RoleData from abu_admin_role where SellerId = -1 and RoleName = '超级管理员'"
	supers, _ := db.Conn().Query(sql)
	hassuper := false
	for supers != nil && supers.Next() {
		hassuper = true
		var roledata string
		supers.Scan(&roledata)
		if roledata != fullauth {
			sql = "select Id,SellerId,RoleName,RoleData from abu_admin_role"
			roles, _ := db.Conn().Query(sql)
			for roles.Next() {
				var roleid int
				var sellerid int
				var rolename string
				var roledata string
				roles.Scan(&roleid, &sellerid, &rolename, &roledata)
				if sellerid == -1 && rolename == "超级管理员" {
					continue
				}
				jnewdata := make(map[string]interface{})
				json.Unmarshal([]byte(fullauth), &jnewdata)
				clean_auth(jnewdata)
				jrdata := make(map[string]interface{})
				json.Unmarshal([]byte(roledata), &jrdata)
				for k, v := range jrdata {
					set_auth(k, jnewdata, v.(map[string]interface{}))
				}
				newauthbyte, _ := json.Marshal(&jnewdata)
				sql = "update abu_admin_role set RoleData = ? where id = ?"
				db.Conn().Exec(sql, string(newauthbyte), roleid)
			}
			roles.Close()
		}
		sql = "update abu_admin_role set RoleData = ? where RoleName = '超级管理员'"
		db.Conn().Exec(sql, fullauth)
	}
	if supers != nil {
		supers.Close()
	}
	if !hassuper {
		sql = "insert into abu_admin_role(RoleName,SellerId,Parent,RoleData)values(?,?,?,?)"
		db.Conn().Exec(sql, "超级管理员", -1, "god", fullauth)
	}
}

func clean_auth(node map[string]interface{}) {
	for k, v := range node {
		if strings.Index(reflect.TypeOf(v).Name(), "float") >= 0 {
			node[k] = 0
		} else {
			clean_auth(v.(map[string]interface{}))
		}
	}
}

func set_auth(parent string, newdata map[string]interface{}, node map[string]interface{}) {
	for k, v := range node {
		if strings.Index(reflect.TypeOf(v).Name(), "float") >= 0 {
			if InterfaceToFloat64(v) != 1 {
				continue
			}
			path := strings.Split(parent, ".")
			if len(path) == 0 {
				continue
			}
			fk, fok := newdata[path[0]]
			if !fok {
				continue
			}
			var pn *interface{} = &fk
			var finded bool = true
			for i := 1; i < len(path); i++ {
				tk := path[i]
				tv, ok := (*pn).(map[string]interface{})[tk]
				if !ok {
					finded = false
					break
				}
				pn = &tv
			}
			if finded {
				(*pn).(map[string]interface{})[k] = 1
			}
		} else {
			set_auth(parent+"."+k, newdata, v.(map[string]interface{}))
		}
	}
}
