package abugo

type GameCallback func()

type GameServer struct {
	game_thread chan GameCallback
	db          *AbuDb
	redis       *AbuRedis
}

func (c *GameServer) Init(db *AbuDb, redis *AbuRedis) {
	c.db = db
	c.redis = redis
	c.game_thread = make(chan GameCallback, 100000)
	go func() {
		for {
			v, ok := <-c.game_thread
			if ok {
				v()
			}
		}
	}()
}
