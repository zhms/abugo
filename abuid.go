package abugo

import (
	"fmt"
	"sync"
	"time"

	uuid "github.com/satori/go.uuid"
)

const (
	snow_nodeBits  uint8 = 10
	snow_stepBits  uint8 = 12
	snow_nodeMax   int64 = -1 ^ (-1 << snow_nodeBits)
	snow_stepMax   int64 = -1 ^ (-1 << snow_stepBits)
	snow_timeShift uint8 = snow_nodeBits + snow_stepBits
	snow_nodeShift uint8 = snow_stepBits
)

var snow_epoch int64 = 1514764800000
var inited = false

type snowflake struct {
	mu        sync.Mutex
	timestamp int64
	node      int64
	step      int64
}

func (this *snowflake) GetId() int64 {
	this.mu.Lock()
	defer this.mu.Unlock()
	now := time.Now().UnixNano() / 1e6
	if this.timestamp == now {
		this.step++
		if this.step > snow_stepMax {
			for now <= this.timestamp {
				now = time.Now().UnixNano() / 1e6
			}
		}
	} else {
		this.step = 0
	}
	this.timestamp = now
	result := (now-snow_epoch)<<snow_timeShift | (this.node << snow_nodeShift) | (this.step)
	return result
}

type IdWorker interface {
	GetId() int64
}

var idworker IdWorker

func NewIdWorker(node int64) {
	if node < 0 || node > snow_nodeMax {
		panic(fmt.Sprintf("snowflake节点必须在0-%d之间", node))
	}
	snowflakeIns := &snowflake{
		timestamp: 0,
		node:      node,
		step:      0,
	}
	idworker = snowflakeIns
	inited = true
}

func AbuId() int64 {
	if !inited {
		return 0
	}
	return idworker.GetId()
}

func AbuGuid() string {
	id := uuid.NewV4()
	return id.String()
}
