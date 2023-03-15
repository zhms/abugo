package abugo

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/viper"
)

type GameCallback func()

type GameUserData struct {
	SellerId   int
	ChannelId  int
	UserId     int
	NickName   string
	Amount     float64
	BankAmount float64
}

type UserData struct {
	BaseData       GameUserData
	Connection     int64
	ReconnectToken string
	HeartBeatCount int
}

type GameMsgCallback func(int, *map[string]interface{})
type GameUserComeCallback func(int)
type GameUserLeaveCallback func(int)
type GameServer struct {
	project           string
	module            string
	game_thread       chan GameCallback
	db                *AbuDb
	redis             *AbuRedis
	http              *AbuHttp
	msgcallbacks      sync.Map
	usercomecallback  GameUserComeCallback
	userleavecallback GameUserLeaveCallback
	user_conn         sync.Map
	conn_user         sync.Map
	gameid            int
	roomlevel         int
	serverid          int
}

func (c *GameServer) Init() {
	Init()
	c.project = Project()
	c.module = Module()

	c.db = new(AbuDb)
	c.db.Init("server.db")
	c.redis = new(AbuRedis)
	c.redis.Init("server.redis")
	c.http = new(AbuHttp)
	c.http.Init("server.http")
	c.http.InitWs("/capi/ws")

	c.gameid = viper.GetInt("server.gameid")
	c.roomlevel = viper.GetInt("server.roomlevel")
	c.serverid = viper.GetInt("server.serverid")
	c.game_thread = make(chan GameCallback, 100000)
	c.http.WsDefaultMsgCallback(c.default_msg_callback)
	c.http.WsAddCloseCallback(c.onwsclose)
	go c.heart_beat()
	go c.game_runner()
}

func (c *GameServer) game_invoke(callback GameCallback) {
	c.game_thread <- callback
}

func (c *GameServer) onwsclose(conn int64) {
	userdata, cbok := c.conn_user.Load(conn)
	if cbok {
		c.conn_user.Delete(conn)
		c.user_conn.Delete(userdata.(*UserData).BaseData.UserId)
		c.game_invoke(func() {
			if c.userleavecallback != nil {
				c.userleavecallback(userdata.(*UserData).BaseData.UserId)
			}
		})
	}
}

func (c *GameServer) default_msg_callback(conn int64, msgid string, data interface{}) {
	mdata := data.(map[string]interface{})
	if msgid == `login` {
		token := GetMapString(&mdata, "Token")
		rediskey := fmt.Sprintf("%s:hall:token:%s", c.project, token)
		redisdata := c.redis.Get(rediskey)
		if redisdata == nil {
			c.http.WsSendMsg(conn, "login", H{"errmsg": "登录失败,token不存在"})
		} else {
			jdata := map[string]interface{}{}
			json.Unmarshal(redisdata.([]byte), &jdata)
			gameid := GetMapInt(&jdata, "GameId")
			roomlevel := GetMapInt(&jdata, "RoomLevel")
			serverid := GetMapInt(&jdata, "ServerId")
			if c.gameid != int(gameid) || c.roomlevel != int(roomlevel) || c.serverid != int(serverid) {
				c.http.WsSendMsg(conn, "login", H{"errmsg": "登录失败,登录信息不匹配"})
			} else {
				c.redis.Del(rediskey)
				UserId := GetMapInt(&jdata, "UserId")
				useridrediskey := fmt.Sprintf("%s:hall:user:data:%d", c.project, UserId)
				redisuserdata := c.redis.HGetAll(useridrediskey)
				userdata := UserData{}
				userdata.Connection = conn
				userdata.BaseData.SellerId = int(GetMapInt(redisuserdata, "SellerId"))
				userdata.BaseData.ChannelId = int(GetMapInt(redisuserdata, "ChannelId"))
				userdata.BaseData.UserId = int(UserId)
				userdata.BaseData.Amount = GetMapFloat64(redisuserdata, "Amount")
				userdata.BaseData.BankAmount = GetMapFloat64(redisuserdata, "BankAmount")
				userdata.BaseData.NickName = GetMapString(redisuserdata, "NickName")
				userdata.ReconnectToken = AbuGuid()
				c.conn_user.Store(conn, &userdata)
				c.user_conn.Store(userdata.BaseData.UserId, &userdata)
				c.SendMsgToUser(userdata.BaseData.UserId, "login", H{
					"SellerId":  userdata.BaseData.SellerId,
					"ChannelId": userdata.BaseData.ChannelId,
					"UserId":    userdata.BaseData.UserId,
					"Amount":    userdata.BaseData.Amount,
					"NickName":  userdata.BaseData.NickName,
				})
				c.game_invoke(func() {
					if c.usercomecallback != nil {
						c.usercomecallback(userdata.BaseData.UserId)
					}
				})
			}
		}
	} else if msgid == "heartbeat" {
		value, ok := c.conn_user.Load(conn)
		if ok {
			v := value.(*UserData)
			v.HeartBeatCount = 0
		}
	}
}

func (c *GameServer) game_runner() {
	for {
		v, ok := <-c.game_thread
		if ok {
			v()
		}
	}
}

func (c *GameServer) heart_beat() {
	for {
		c.conn_user.Range(func(key, value any) bool {
			v := value.(*UserData)
			if v.HeartBeatCount >= 5 {
				c.http.WsClose(key.(int64))
				c.onwsclose(key.(int64))
			} else {
				v.HeartBeatCount++
				c.http.WsSendMsg(key.(int64), "heartbeat", H{"Count": v.HeartBeatCount})
			}
			return true
		})
		time.Sleep(time.Second * 2)
	}
}

func (c *GameServer) AddUserComeCallback(callback GameUserComeCallback) {
	c.usercomecallback = callback
}

func (c *GameServer) AddUserLeaveCallback(callback GameUserLeaveCallback) {

	c.userleavecallback = callback

}

func (c *GameServer) AddMsgCallback(msgid string, callback GameMsgCallback) {
	c.msgcallbacks.Store(msgid, callback)
}

func (c *GameServer) RemoveMsgCallback(msgid string) {
	c.msgcallbacks.Delete(msgid)
}

func (c *GameServer) SendMsgToUser(UserId int, msgid string, data interface{}) {
	userdata, ok := c.user_conn.Load(UserId)
	if ok {
		c.http.WsSendMsg(userdata.(*UserData).Connection, msgid, data)
	}
}

func (c *GameServer) SendMsgToAll(msgid string, data interface{}) {
	c.conn_user.Range(func(key, value any) bool {
		v := value.(*UserData)
		c.http.WsSendMsg(v.Connection, msgid, data)
		return true
	})
}

func (c *GameServer) KickOutUser(UserId int) {
	value, ok := c.user_conn.Load(UserId)
	if ok {
		conn := value.(*UserData).Connection
		c.http.WsClose(conn)
		c.onwsclose(conn)
	}
}

func (c *GameServer) GetUserData(UserId int) *GameUserData {
	value, ok := c.user_conn.Load(UserId)
	if ok {
		return &value.(*UserData).BaseData
	} else {
		return nil
	}
}
