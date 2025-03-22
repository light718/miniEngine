package engine

import "log"

type IDispatchEngineEvent interface {
	//ws事件
	OnWSocketConnEvent(sid int64, remote string)
	OnWSocketRecvEvent(sid int64, message []byte)
	OnWSocketCloseEvent(sid int64)
	//定时器事件
	OnTimerEvent(id1, id2, id3, id4, id5 int, para1, para2 interface{})
}

// 服务上下文类型
const (
	ENGINE_NET_WS_CONN = iota
	ENGINE_NET_WS_RECV
	ENGINE_NET_WS_CLOSE
	ENGINE_TIMER
)

type wsContext struct {
	sid     int64
	remote  string
	message []byte
}

type timerContext struct {
	id1, id2, id3, id4, id5 int
	para1, para2            interface{}
}

// 上下文
type ServiceEngineContext struct {
	//服务上下文类型
	OptType int
	//ws事件
	wsctx wsContext
	//定时器事件
	timerctx timerContext
}

type DispatchEngine struct {
	wsEngine    *WebSocketEngine
	timerEngine *TimerEngine
	iDispatch   IDispatchEngineEvent
	ch          chan ServiceEngineContext
	stop        chan struct{}
	release     chan struct{}
}

func New(wsaddr string, i IDispatchEngineEvent, cache int) (engine *DispatchEngine) {
	engine = &DispatchEngine{
		iDispatch: i,
		ch:        make(chan ServiceEngineContext, cache),
		stop:      make(chan struct{}),
	}
	engine.wsEngine = NewWebSocketEngine(wsaddr, engine, cache)
	engine.timerEngine = NewTimerEngine(engine, cache)
	return
}

func (engine *DispatchEngine) Start() {
	go engine.Dispatch()
}

func (engine *DispatchEngine) Stop() {
	engine.wsEngine.Stop()
	engine.timerEngine.Stop()
	engine.stop <- struct{}{}
	<-engine.release
}

func (engine *DispatchEngine) OnWebSocketConnEvent(sid int64, remote string) {
	engine.ch <- ServiceEngineContext{
		OptType: ENGINE_NET_WS_CONN,
		wsctx: wsContext{
			sid:    sid,
			remote: remote,
		},
	}
}

func (engine *DispatchEngine) OnWebSocketRecvEvent(sid int64, message []byte) {
	engine.ch <- ServiceEngineContext{
		OptType: ENGINE_NET_WS_RECV,
		wsctx: wsContext{
			sid:     sid,
			message: message,
		},
	}
}

func (engine *DispatchEngine) OnWebSocketCloseEvent(sid int64) {
	engine.ch <- ServiceEngineContext{
		OptType: ENGINE_NET_WS_CLOSE,
		wsctx: wsContext{
			sid: sid,
		},
	}
}

func (engine *DispatchEngine) OnTimerEvent(id1, id2, id3, id4, id5 int, para1, para2 interface{}) {
	engine.ch <- ServiceEngineContext{
		OptType: ENGINE_TIMER,
		timerctx: timerContext{
			id1:   id1,
			id2:   id2,
			id3:   id3,
			id4:   id4,
			id5:   id5,
			para1: para1,
			para2: para2,
		},
	}
}

func (engine *DispatchEngine) Dispatch() {
	defer func() {
		if err := recover(); err != nil {
			log.Panicf("DispatchEngine Dispatch error:%v", err)
		}
	}()
LOOP:
	for {
		select {
		case ctx := <-engine.ch:
			switch ctx.OptType {
			case ENGINE_NET_WS_CONN:
				engine.iDispatch.OnWSocketConnEvent(ctx.wsctx.sid, ctx.wsctx.remote)
			case ENGINE_NET_WS_RECV:
				engine.iDispatch.OnWSocketRecvEvent(ctx.wsctx.sid, ctx.wsctx.message)
			case ENGINE_NET_WS_CLOSE:
				engine.iDispatch.OnWSocketCloseEvent(ctx.wsctx.sid)
			case ENGINE_TIMER:
				engine.iDispatch.OnTimerEvent(ctx.timerctx.id1, ctx.timerctx.id2, ctx.timerctx.id3, ctx.timerctx.id4, ctx.timerctx.id5, ctx.timerctx.para1, ctx.timerctx.para2)
			}
		case <-engine.stop:
			break LOOP
		}
	}
	engine.release <- struct{}{}
}
