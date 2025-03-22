package engine

import (
	"container/heap"
	"log"
	"time"
)

// 上下文事件定义
const (
	TIMER_ENGINE_EVENT_ADD = iota
	TIMER_ENGINE_EVENT_DEL
	TIMER_ENGINE_EVENT_FIX
)

type TimerEngineContext struct {
	//事件类型
	evType int
	//定时器类型ID
	id1 int
	id2 int
	id3 int
	id4 int
	id5 int
	//过期时间
	expire time.Duration
	//附带参数
	para1 interface{}
	para2 interface{}
}

type TimerItem struct {
	deadline time.Time //过期
	index    int
	id1      int
	id2      int
	id3      int
	id4      int
	id5      int
	para1    interface{}
	para2    interface{}
}

type TimerItems []TimerItem
type TimerHeap struct {
	items TimerItems
}

func NewTimerHeap() (h *TimerHeap) {
	h = &TimerHeap{
		items: make(TimerItems, 0),
	}
	heap.Init(h)
	return
}

func (h TimerHeap) Len() int { return len(h.items) }
func (h TimerHeap) Less(i, j int) bool {
	return h.items[i].deadline.Before(h.items[j].deadline)
}
func (h TimerHeap) Swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
	h.items[i].index = i
	h.items[j].index = j
}

func (h *TimerHeap) Push(x interface{}) {
	n := len(h.items)
	item := x.(TimerItem)
	item.index = n
	h.items = append(h.items, item)
}

func (h *TimerHeap) Pop() interface{} {
	old := h.items
	n := len(old)
	item := old[n-1]
	item.index = -1
	h.items = old[0 : n-1]
	return item
}

func (h *TimerHeap) Add(id1, id2, id3, id4, id5 int, expire time.Duration, para1, para2 interface{}) {
	item := TimerItem{
		deadline: time.Now().Add(expire),
		id1:      id1,
		id2:      id2,
		id3:      id3,
		id4:      id4,
		id5:      id5,
		para1:    para1,
		para2:    para2,
	}
	heap.Push(h, item)
}

func (h *TimerHeap) Del(id1, id2, id3, id4, id5 int) {
	for i := 0; i < len(h.items); i++ {
		if h.items[i].id1 == id1 && h.items[i].id2 == id2 && h.items[i].id3 == id3 && h.items[i].id4 == id4 && h.items[i].id5 == id5 {
			heap.Remove(h, h.items[i].index)
			break
		}
	}
}

func (h *TimerHeap) Fix(id1, id2, id3, id4, id5 int, expire time.Duration) {
	for i := 0; i < len(h.items); i++ {
		if h.items[i].id1 == id1 && h.items[i].id2 == id2 && h.items[i].id3 == id3 && h.items[i].id4 == id4 && h.items[i].id5 == id5 {
			h.items[i].deadline = time.Now().Add(expire)
			heap.Fix(h, h.items[i].index)
			break
		}
	}
}

// 定时器引擎
type TimerEngine struct {
	timerHeap *TimerHeap
	dispatch  *DispatchEngine
	ch        chan TimerEngineContext
	stop      chan struct{}
	release   chan struct{}
}

func NewTimerEngine(d *DispatchEngine, cache int) (engine *TimerEngine) {
	engine = &TimerEngine{
		dispatch:  d,
		timerHeap: NewTimerHeap(),
		ch:        make(chan TimerEngineContext, cache),
		stop:      make(chan struct{}),
		release:   make(chan struct{}),
	}

	return
}

func (engine *TimerEngine) Start() {
	go engine.Dispatch()
}

func (engine *TimerEngine) Stop() {
	engine.stop <- struct{}{}
	<-engine.release
}

func (engine *TimerEngine) Add(id1, id2, id3, id4, id5 int, expire time.Duration, para1, para2 interface{}) {
	engine.ch <- TimerEngineContext{
		evType: TIMER_ENGINE_EVENT_ADD,
		id1:    id1,
		id2:    id2,
		id3:    id3,
		id4:    id4,
		id5:    id5,
		expire: expire,
		para1:  para1,
		para2:  para2,
	}
}

func (engine *TimerEngine) Del(id1, id2, id3, id4, id5 int) {
	engine.ch <- TimerEngineContext{
		evType: TIMER_ENGINE_EVENT_DEL,
		id1:    id1,
		id2:    id2,
		id3:    id3,
		id4:    id4,
		id5:    id5,
	}
}

func (engine *TimerEngine) Fix(id1, id2, id3, id4, id5 int, expire time.Duration) {
	engine.ch <- TimerEngineContext{
		evType: TIMER_ENGINE_EVENT_FIX,
		id1:    id1,
		id2:    id2,
		id3:    id3,
		id4:    id4,
		id5:    id5,
		expire: expire,
	}
}

func (engine *TimerEngine) Dispatch() {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("TimerEngine error:%v", err)
		}
	}()
	timer := time.NewTimer(time.Millisecond)
	deadline := time.Now()
LOOP:
	for {
		select {
		case ctx := <-engine.ch:
			{
				switch ctx.evType {
				case TIMER_ENGINE_EVENT_ADD:
					engine.timerHeap.Add(ctx.id1, ctx.id2, ctx.id3, ctx.id4, ctx.id5, ctx.expire, ctx.para1, ctx.para2)
				case TIMER_ENGINE_EVENT_DEL:
					engine.timerHeap.Del(ctx.id1, ctx.id2, ctx.id3, ctx.id4, ctx.id5)
				case TIMER_ENGINE_EVENT_FIX:
					engine.timerHeap.Fix(ctx.id1, ctx.id2, ctx.id3, ctx.id4, ctx.id5, ctx.expire)
				}
				if engine.timerHeap.Len() > 0 {
					if !deadline.Equal(engine.timerHeap.items[0].deadline) {
						if !timer.Stop() {
							select {
							case <-timer.C:
							default:
							}
						}
						deadline = engine.timerHeap.items[0].deadline
						timer.Reset(time.Until(engine.timerHeap.items[0].deadline))
					}
				} else {
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
				}
			}
		case <-timer.C:
			{
				now := time.Now()
				for engine.timerHeap.Len() > 0 {
					item := &engine.timerHeap.items[0]
					if item.deadline.Before(now) {
						engine.dispatch.OnTimerEvent(item.id1, item.id2, item.id3, item.id4, item.id5, item.para1, item.para2)
						heap.Pop(engine.timerHeap)
					} else {
						break
					}
				}
				if engine.timerHeap.Len() > 0 {
					deadline = engine.timerHeap.items[0].deadline
					timer.Reset(deadline.Sub(now))
				}
			}
		case <-engine.stop:
			{
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				break LOOP
			}
		}
	}
	engine.release <- struct{}{}
}
