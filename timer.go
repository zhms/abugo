package abugo

import (
	"sync"
	"time"
)

type TimerCallback func(int64)
type timernode struct {
	TimerId    int64
	TiggerTime int64
	Delay      int
	Count      int
	Callback   TimerCallback
	Prev       *timernode
	Next       *timernode
}

var maptimers sync.Map
var timerhead *timernode
var timerid int64 = 1

var timerlock sync.Mutex

func timertick() {
	for {
		if timerhead == nil {
			time.Sleep(time.Millisecond * 1)
			continue
		}
		nottime := time.Now().UnixNano() / 1e6
		if timerhead.TiggerTime > nottime {
			time.Sleep(time.Millisecond * 1)
			continue
		}
		timerlock.Lock()
		node := timerhead
		timerhead = timerhead.Next
		if timerhead != nil {
			timerhead.Prev = nil
		}
		timerlock.Unlock()
		if node.Count > 0 {
			node.Count--
		}
		if node.Count != 0 {
			node.TiggerTime += int64(node.Delay)
			if timerhead == nil {
				timerhead = node
			} else {
				td := timerhead
				for {
					if node.TiggerTime <= td.TiggerTime && td.Next != nil {
						td = td.Next
					} else {
						node.Next = td.Next
						td.Next = node
						node.Prev = td
						break
					}
				}
			}
		} else {
			maptimers.Delete(node.TimerId)
		}
		node.Callback(node.TimerId)
	}
}

func AddTimerInterval(delay int, count int, callback TimerCallback) int64 {
	if delay < 0 || count == 0 || callback == nil {
		return 0
	}
	timerlock.Lock()
	defer func() {
		timerlock.Unlock()
	}()
	tid := timerid
	timerid++
	node := timernode{}
	node.TimerId = tid
	node.Delay = delay
	node.Count = count
	node.TiggerTime = time.Now().UnixNano()/1e6 + int64(delay)
	node.Callback = callback
	if timerhead == nil {
		timerhead = &node
	} else {
		td := timerhead
		for {
			if node.TiggerTime <= td.TiggerTime && td.Next != nil {
				td = td.Next
			} else {
				node.Next = td.Next
				td.Next = &node
				node.Prev = td
				break
			}
		}
	}
	maptimers.Store(node.TimerId, &node)
	return tid
}

func AddTimer(delay int, callback TimerCallback) int64 {
	return AddTimerInterval(delay, 1, callback)
}

func KillTimer(tid int64) {
	timerlock.Lock()
	defer func() {
		timerlock.Unlock()
	}()
	mtd, ok := maptimers.Load(tid)
	if !ok {
		return
	}
	td := mtd.(*timernode)
	if td.Prev != nil {
		td.Prev.Next = td.Next
	}
	if td.Next != nil {
		td.Next.Prev = td.Prev
	}
	maptimers.Delete(tid)
}
