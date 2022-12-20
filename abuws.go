package abugo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/beego/beego/logs"
	"github.com/gorilla/websocket"
)

type AbuWsCallback func(int64)
type AbuWsMsgCallback func(int64, interface{})
type AbuWebsocket struct {
	upgrader         websocket.Upgrader
	idx_conn         sync.Map
	conn_idx         sync.Map
	connect_callback AbuWsCallback
	close_callback   AbuWsCallback
	msgtype          sync.Map
	msg_callback     sync.Map
}

type abumsgdata struct {
	MsgId string      `json:"msgid"`
	Data  interface{} `json:"data"`
}

func (c *AbuWebsocket) Init(prefix string) {
	port := GetConfigInt(fmt.Sprint(prefix, ".port"), true, 0)
	c.upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	go func() {
		http.HandleFunc("/", c.home)
		bind := fmt.Sprint("0.0.0.0:", port)
		http.ListenAndServe(bind, nil)
	}()
	logs.Debug("websocket listen:", port)
}

func (c *AbuWebsocket) home(w http.ResponseWriter, r *http.Request) {
	conn, err := c.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logs.Error(err)
		return
	}
	defer conn.Close()
	id := AbuId()
	c.idx_conn.Store(id, conn)
	c.conn_idx.Store(conn, id)
	if c.connect_callback != nil {
		c.connect_callback(id)
	}
	for {
		mt, message, err := conn.ReadMessage()
		c.msgtype.Store(id, mt)
		if err != nil {
			break
		}
		md := abumsgdata{}
		err = json.Unmarshal(message, &md)
		if err == nil {
			callback, cbok := c.msg_callback.Load(md.MsgId)
			if cbok {
				cb := callback.(AbuWsMsgCallback)
				cb(id, md.Data)
			}
		}
	}
	_, ccerr := c.idx_conn.Load(id)
	if ccerr {
		c.idx_conn.Delete(id)
		c.conn_idx.Delete(conn)
		if c.close_callback != nil {
			c.close_callback(id)
		}
	}
}

func (c *AbuWebsocket) SendMsg(id int64, msgid string, data interface{}) {
	iconn, connok := c.idx_conn.Load(id)
	imt, mtok := c.msgtype.Load(id)
	if mtok && connok {
		conn := iconn.(*websocket.Conn)
		mt := imt.(int)
		msg := abumsgdata{msgid, data}
		msgbyte, jerr := json.Marshal(msg)
		if jerr == nil {
			werr := conn.WriteMessage(mt, msgbyte)
			if werr != nil {
			}
		}
	}
}

func (c *AbuWebsocket) Close(id int64) {
	iconn, connok := c.idx_conn.Load(id)
	if connok {
		conn := iconn.(*websocket.Conn)
		c.conn_idx.Delete(conn)
		c.idx_conn.Delete(id)
		c.msgtype.Delete(id)
		conn.Close()
	}
}

func (c *AbuWebsocket) Connect(host string, callback AbuWsCallback) {
	go func() {
		conn, _, err := websocket.DefaultDialer.Dial(host, nil)
		if err != nil {
			callback(0)
			return
		}
		defer conn.Close()
		id := AbuId()
		c.idx_conn.Store(id, conn)
		c.conn_idx.Store(conn, id)
		callback(id)
		for {
			mt, message, err := conn.ReadMessage()
			c.msgtype.Store(id, mt)
			if err != nil {
				break
			}
			md := abumsgdata{}
			err = json.Unmarshal(message, &md)
			if err == nil {
				callback, cbok := c.msg_callback.Load(md.MsgId)
				if cbok {
					cb := callback.(AbuWsMsgCallback)
					cb(id, md.Data)
				}
			}
		}
		_, ccerr := c.idx_conn.Load(id)
		if ccerr {
			c.idx_conn.Delete(id)
			c.conn_idx.Delete(conn)
			if c.close_callback != nil {
				c.close_callback(id)
			}
		}
	}()
}

func (c *AbuWebsocket) AddConnectCallback(callback AbuWsCallback) {
	c.connect_callback = callback
}

func (c *AbuWebsocket) AddMsgCallback(msgid string, callback AbuWsMsgCallback) {
	c.msg_callback.Store(msgid, callback)
}

func (c *AbuWebsocket) AddCloseCallback(callback AbuWsCallback) {
	c.close_callback = callback
}
