package abugo

import (
	"fmt"
	"sync"
)

type GameCallback func()

type GameMsgCallback func(int, *map[string]interface{})
type GameUserComeCallback func(int)
type GameUserLeaveCallback func(int)
type GameServer struct {
	game_thread       chan GameCallback
	db                *AbuDb
	redis             *AbuRedis
	http              *AbuHttp
	msgcallbacks      map[string]GameMsgCallback
	usercomecallback  GameUserComeCallback
	userleavecallback GameUserLeaveCallback
	lock              sync.Mutex
	user_conn         sync.Map
	conn_user         sync.Map
}

func (c *GameServer) Init(http *AbuHttp, db *AbuDb, redis *AbuRedis) {
	c.lock = sync.Mutex{}
	c.db = db
	c.redis = redis
	c.http = http
	c.game_thread = make(chan GameCallback, 100000)
	http.WsAddCloseCallback(c.onwsclose)
	go func() {
		for {
			v, ok := <-c.game_thread
			if ok {
				v()
			}
		}
	}()
}

func (c *GameServer) game_invoke(callback GameCallback) {
	c.game_thread <- callback
}

func (c *GameServer) onwsclose(conn int64) {
	userid, cbok := c.conn_user.Load(conn)
	if cbok {
		c.conn_user.Delete(conn)
		c.user_conn.Delete(userid.(int))
		if c.userleavecallback != nil {
			c.userleavecallback(userid.(int))
		}
	}
}

func (c *GameServer) default_msg_callback(conn int64, msgid string, data interface{}) {
	fmt.Println(conn, msgid, data)
}

func (c *GameServer) AddUserComeCallback(callback GameUserComeCallback) {
	c.lock.Lock()
	c.usercomecallback = callback
	c.lock.Unlock()
}

func (c *GameServer) AddUserLeaveCallback(callback GameUserLeaveCallback) {
	c.lock.Lock()
	c.userleavecallback = callback
	c.lock.Unlock()
}

func (c *GameServer) AddMsgCallback(msgid string, callback GameMsgCallback) {
	c.lock.Lock()
	c.msgcallbacks[msgid] = callback
	c.lock.Unlock()
}

func (c *GameServer) RemoveMsgCallback(msgid string) {
	c.lock.Lock()
	delete(c.msgcallbacks, msgid)
	c.lock.Unlock()
}

func (c *GameServer) SendMsgToUser(UserId int, data interface{}) {
	c.lock.Lock()
	c.lock.Unlock()
}

func (c *GameServer) SendMsgToAll(data interface{}) {
	c.lock.Lock()
	c.lock.Unlock()
}

func (c *GameServer) KickOutUser(UserId int) {
	c.lock.Lock()
	c.lock.Unlock()
}
