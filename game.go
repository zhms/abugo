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
	ChairId    int
	Ready      bool
}

type GameInfo struct {
	DeskCount     int //多少个桌子
	ChairCount    int //桌子多少个座位
	MinStartCount int //每个桌子至少需要几人才可以开始游戏
	/*
		1.老虎机,一个人一个桌子
		2.百人游戏,所有人一个桌子
		3.斗地主,牛牛,多人一个桌子
		4.捕鱼,随时来,随时走,随时开始
	*/
	MakeType int
}

type UserData struct {
	BaseData       GameUserData
	Connection     int64
	ReconnectToken string
	HeartBeatCount int
	Desk           *GameDesk
}

type IServer interface {
	SendMsgToUser(UserId int, msgid string, data interface{})
	SendMsgToAll(msgid string, data interface{})
	KickOutUser(UserId int)
	GetUserData(UserId int) *GameUserData
}

type IGameScene interface {
	Init(IServer)
	UserEnter(int)
	UserLeave(int)
	Release()
}

type AllocDeskCallback func() IGameScene

type GameDesk struct {
	DeskId       int
	GameScene    IGameScene
	GameSrv      *GameServer
	users        []*UserData
	usercount    int
	msgcallbacks sync.Map
}

type ReadyUser struct {
	User *UserData
	Prev *ReadyUser
	Next *ReadyUser
}

type GameMsgCallback func(int, *map[string]interface{})
type GameUserComeCallback func(int)
type GameUserLeaveCallback func(int)
type GameServer struct {
	project        string
	module         string
	game_thread    chan GameCallback
	db             *AbuDb
	redis          *AbuRedis
	http           *AbuHttp
	user_conn      sync.Map
	conn_user      sync.Map
	gameid         int
	roomlevel      int
	serverid       int
	gameinfo       GameInfo
	newgamescene   AllocDeskCallback
	desks          sync.Map
	deskid         int
	readyuserhead  *ReadyUser
	readyusertail  *ReadyUser
	readyusercount int
	locker         sync.Mutex
}

func (this *GameServer) Init(gameinfo GameInfo, callback AllocDeskCallback) {
	Init()
	this.project = Project()
	this.module = Module()
	this.gameinfo = gameinfo
	this.newgamescene = callback
	this.deskid = 1
	this.readyusercount = 0

	this.db = new(AbuDb)
	this.db.Init("server.db")
	this.redis = new(AbuRedis)
	this.redis.Init("server.redis")
	this.http = new(AbuHttp)
	this.http.Init("server.http")
	this.http.InitWs("/capi/ws")

	this.gameid = viper.GetInt("server.gameid")
	this.roomlevel = viper.GetInt("server.roomlevel")
	this.serverid = viper.GetInt("server.serverid")
	this.game_thread = make(chan GameCallback, 100000)
	this.http.WsDefaultMsgCallback(this.default_msg_callback)
	this.http.WsAddCloseCallback(this.onwsclose)
	go this.game_runner()
	//go this.heart_beat()
	go func() {
		this.make_desk()
		time.Sleep(time.Second * 2)
	}()
}

func (this *GameServer) game_invoke(callback GameCallback) {
	this.game_thread <- callback
}

func (this *GameServer) onwsclose(conn int64) {
	userdata, ok := this.conn_user.Load(conn)
	if ok {
		this.conn_user.Delete(conn)
		this.user_conn.Delete(userdata.(*UserData).BaseData.UserId)
		ud := userdata.(*UserData)
		if ud.Desk != nil {
			ud.Desk.users[ud.BaseData.ChairId] = nil
			ud.Desk.usercount--
			this.game_invoke(func() {
				ud.Desk.GameScene.UserLeave(ud.BaseData.ChairId)
				if this.gameinfo.MakeType == 1 {
					if ud.Desk.usercount == 0 {
						this.desks.Delete(ud.Desk.DeskId)
						ud.Desk.GameScene.Release()
					}
				}
			})
		}
	}
}

func (this *GameServer) default_msg_callback(conn int64, msgid string, data interface{}) {
	mdata := data.(map[string]interface{})
	if msgid == `login` {
		token := GetMapString(&mdata, "Token")
		rediskey := fmt.Sprintf("%s:hall:token:%s", this.project, token)
		redisdata := this.redis.Get(rediskey)
		if redisdata == nil {
			this.http.WsSendMsg(conn, "login", H{"errmsg": "登录失败,token不存在"})
		} else {
			jdata := map[string]interface{}{}
			json.Unmarshal(redisdata.([]byte), &jdata)
			gameid := GetMapInt(&jdata, "GameId")
			roomlevel := GetMapInt(&jdata, "RoomLevel")
			serverid := GetMapInt(&jdata, "ServerId")
			if this.gameid != int(gameid) || this.roomlevel != int(roomlevel) || this.serverid != int(serverid) {
				this.http.WsSendMsg(conn, "login", H{"errmsg": "登录失败,登录信息不匹配"})
			} else {
				this.redis.Del(rediskey)
				UserId := GetMapInt(&jdata, "UserId")
				useridrediskey := fmt.Sprintf("%s:hall:user:data:%d", this.project, UserId)
				redisuserdata := this.redis.HGetAll(useridrediskey)
				userdata := UserData{}
				userdata.Connection = conn
				userdata.BaseData.SellerId = int(GetMapInt(redisuserdata, "SellerId"))
				userdata.BaseData.ChannelId = int(GetMapInt(redisuserdata, "ChannelId"))
				userdata.BaseData.UserId = int(UserId)
				userdata.BaseData.Amount = GetMapFloat64(redisuserdata, "Amount")
				userdata.BaseData.BankAmount = GetMapFloat64(redisuserdata, "BankAmount")
				userdata.BaseData.NickName = GetMapString(redisuserdata, "NickName")
				userdata.ReconnectToken = AbuGuid()
				this.conn_user.Store(conn, &userdata)
				this.user_conn.Store(userdata.BaseData.UserId, &userdata)
				this.SendMsgToUser(userdata.BaseData.UserId, "login", H{
					"SellerId":  userdata.BaseData.SellerId,
					"ChannelId": userdata.BaseData.ChannelId,
					"UserId":    userdata.BaseData.UserId,
					"Amount":    userdata.BaseData.Amount,
					"NickName":  userdata.BaseData.NickName,
				})

			}
		}
	} else if msgid == "ready" {
		userdata, ok := this.conn_user.Load(conn)
		if ok {
			this.user_ready(userdata.(*UserData))
		}

	} else if msgid == "unready" {
		userdata, ok := this.conn_user.Load(conn)
		if ok {
			this.user_unready(userdata.(*UserData))
		}

	} else if msgid == "heartbeat" {
		value, ok := this.conn_user.Load(conn)
		if ok {
			v := value.(*UserData)
			v.HeartBeatCount = 0
		}
	} else {
		userdata, ok := this.conn_user.Load(conn)
		if ok {
			ud := userdata.(*UserData)
			if ud.Desk != nil {
				cb, ok := ud.Desk.msgcallbacks.Load(msgid)
				if ok {
					callback := cb.(GameMsgCallback)
					this.game_invoke(func() {
						callback(ud.BaseData.ChairId, &mdata)
					})
				}
			}
		}
	}
}

func (this *GameServer) game_runner() {
	for {
		v, ok := <-this.game_thread
		if ok {
			v()
		}
	}
}

func (this *GameServer) heart_beat() {
	for {
		this.conn_user.Range(func(key, value any) bool {
			v := value.(*UserData)
			if v.HeartBeatCount >= 5 {
				this.http.WsClose(key.(int64))
				this.onwsclose(key.(int64))
			} else {
				v.HeartBeatCount++
				this.http.WsSendMsg(key.(int64), "heartbeat", H{"Count": v.HeartBeatCount})
			}
			return true
		})
		time.Sleep(time.Second * 2)
	}
}

func (this *GameServer) SendMsgToUser(UserId int, msgid string, data interface{}) {
	userdata, ok := this.user_conn.Load(UserId)
	if ok {
		this.http.WsSendMsg(userdata.(*UserData).Connection, msgid, data)
	}
}

func (this *GameServer) SendMsgToAll(msgid string, data interface{}) {
	this.conn_user.Range(func(key, value any) bool {
		v := value.(*UserData)
		this.http.WsSendMsg(v.Connection, msgid, data)
		return true
	})
}

func (this *GameServer) KickOutUser(UserId int) {
	value, ok := this.user_conn.Load(UserId)
	if ok {
		conn := value.(*UserData).Connection
		this.http.WsClose(conn)
		this.onwsclose(conn)
	}
}

func (this *GameServer) GetUserData(UserId int) *GameUserData {
	value, ok := this.user_conn.Load(UserId)
	if ok {
		return &value.(*UserData).BaseData
	} else {
		return nil
	}
}

func (this *GameServer) user_ready(userdata *UserData) {
	this.locker.Lock()
	defer func() {
		this.locker.Unlock()
	}()
	if userdata.BaseData.Ready {
		return
	}
	user := ReadyUser{}
	user.User = userdata
	user.User.BaseData.Ready = true
	if this.readyuserhead == nil {
		this.readyuserhead = &user
		this.readyusertail = this.readyuserhead
		this.readyusercount++
	} else {
		user.Prev = this.readyusertail
		this.readyusertail.Next = &user
		this.readyusercount++
	}
}

func (this *GameServer) user_unready(userdata *UserData) {
	this.locker.Lock()
	defer func() {
		this.locker.Unlock()
	}()
	if this.readyuserhead.User == userdata {
		if this.readyusercount == 1 {
			this.readyusercount = 0
			this.readyuserhead = nil
			this.readyusertail = nil
		} else {
			this.readyuserhead = this.readyuserhead.Next
		}
		this.readyusercount--
	} else if this.readyusertail.User == userdata {
		if this.readyusercount == 1 {
			this.readyusercount = 0
			this.readyuserhead = nil
			this.readyusertail = nil
		} else {
			this.readyusertail.Prev.Next = nil
		}
		this.readyusercount--
	} else {
		node := this.readyuserhead
		for {
			if node == nil {
				break
			}
			if node.User != userdata {
				node = node.Next
			} else {
				node.Next.Prev = node.Prev
				node.Prev.Next = node.Next
				this.readyusercount--
			}
		}
	}
}

func (this *GameServer) make_desk() {
	this.locker.Lock()
	defer func() {
		this.locker.Unlock()
	}()
	if this.readyusercount == 0 {
		return
	}
	if this.gameinfo.MakeType == 1 {
		deskid := this.deskid
		this.deskid++
		desk := GameDesk{}
		desk.GameScene = this.newgamescene()
		desk.GameSrv = this
		desk.users = make([]*UserData, this.gameinfo.ChairCount)
		desk.DeskId = deskid
		desk.GameScene.Init(&desk)
		this.desks.Store(desk.DeskId, &desk)
		desk.users[0] = this.readyuserhead.User
		desk.users[0].BaseData.ChairId = 0
		desk.users[0].Desk = &desk
		this.readyusercount--
		if this.readyusercount == 0 {
			this.readyuserhead = nil
			this.readyusertail = nil
		} else {
			this.readyuserhead = this.readyuserhead.Next
		}
		this.game_invoke(func() {
			desk.GameScene.UserEnter(desk.users[0].BaseData.ChairId)
		})
	}
}

func (this *GameDesk) GetUserData(ChairId int) *GameUserData {
	if this.users[ChairId] == nil {
		return nil
	}
	return &this.users[ChairId].BaseData
}

func (this *GameDesk) KickOutUser(ChairId int) {
	if this.users[ChairId] == nil {
		return
	}
	this.GameSrv.KickOutUser(this.users[ChairId].BaseData.UserId)
}

func (this *GameDesk) AddMsgCallback(msgid string, callback GameMsgCallback) {
	this.msgcallbacks.Store(msgid, callback)
}

func (this *GameDesk) RemoveMsgCallback(msgid string) {
	this.msgcallbacks.Delete(msgid)
}

func (this *GameDesk) SendMsgToUser(UserId int, msgid string, data interface{}) {}
func (this *GameDesk) SendMsgToAll(msgid string, data interface{})              {}
