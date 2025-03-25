package miniEngine

import "log"

// 服务上下文类型
const (
	ENGINE_NET_WS_CONN = iota
	ENGINE_NET_WS_RECV
	ENGINE_NET_WS_CLOSE
	ENGINE_TIMER
)

type (
	//调度引擎事件接口
	IAttemperhEngineEvent interface {
		//ws事件
		OnWSocketConn(sid int64, remote string)
		OnWSocketRecv(sid int64, message []byte)
		OnWSocketClose(sid int64)
		//定时器事件
		OnTimer(id1, id2, id3, id4, id5 int, para1, para2 interface{})
	}
	//websocket事件上下文
	wsContext struct {
		sid     int64
		remote  string
		message []byte
	}
	//定时器事件上下文
	timerContext struct {
		id1, id2, id3, id4, id5 int
		para1, para2            interface{}
	}
	AttemperhEngineContext struct {
		//服务上下文类型
		OptType int
		//ws事件
		wsctx wsContext
		//定时器事件
		timerctx timerContext
	}
	//调度引擎
	AttemperhEngine struct {
		iEvent      IAttemperhEngineEvent
		timerEngine *TimerEngine
		wsEngine    *WebSocketEngine
		ch          chan AttemperhEngineContext
		stop        chan struct{}
		release     chan struct{}
	}
)

func NewAttemperhEngine(wsaddr string, chSize int) (engine *AttemperhEngine) {
	engine = &AttemperhEngine{
		ch:   make(chan AttemperhEngineContext, chSize),
		stop: make(chan struct{}),
	}
	engine.wsEngine = NewWebSocketEngine(wsaddr, engine, chSize)
	engine.timerEngine = NewTimerEngine(engine, chSize)
	return
}

func (engine *AttemperhEngine) SetEvent(i IAttemperhEngineEvent) {
	engine.iEvent = i
}

func (engine *AttemperhEngine) Start() {
	engine.timerEngine.start()
	engine.wsEngine.start()
	go engine.dispatch()
}

func (engine *AttemperhEngine) Stop() {
	engine.wsEngine.stop()
	engine.timerEngine.stop()
	engine.stop <- struct{}{}
	<-engine.release
	close(engine.stop)
	close(engine.release)
}

func (engine *AttemperhEngine) doWebSocketConn(sid int64, remote string) {
	engine.ch <- AttemperhEngineContext{
		OptType: ENGINE_NET_WS_CONN,
		wsctx: wsContext{
			sid:    sid,
			remote: remote,
		},
	}
}

func (engine *AttemperhEngine) doWebSocketRecv(sid int64, message []byte) {
	engine.ch <- AttemperhEngineContext{
		OptType: ENGINE_NET_WS_RECV,
		wsctx: wsContext{
			sid:     sid,
			message: message,
		},
	}
}

func (engine *AttemperhEngine) doWebSocketClose(sid int64) {
	engine.ch <- AttemperhEngineContext{
		OptType: ENGINE_NET_WS_CLOSE,
		wsctx: wsContext{
			sid: sid,
		},
	}
}

func (engine *AttemperhEngine) doTimer(id1, id2, id3, id4, id5 int, para1, para2 interface{}) {
	engine.ch <- AttemperhEngineContext{
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

func (engine *AttemperhEngine) TimerEngine() *TimerEngine {
	return engine.timerEngine
}

func (engine *AttemperhEngine) WebSocketEngine() *WebSocketEngine {
	return engine.wsEngine
}

func (engine *AttemperhEngine) dispatch() {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("DispatchEngine Dispatch error:%v", err)
		}
	}()
LOOP:
	for {
		select {
		case ctx := <-engine.ch:
			switch ctx.OptType {
			case ENGINE_NET_WS_CONN:
				engine.iEvent.OnWSocketConn(ctx.wsctx.sid, ctx.wsctx.remote)
			case ENGINE_NET_WS_RECV:
				engine.iEvent.OnWSocketRecv(ctx.wsctx.sid, ctx.wsctx.message)
			case ENGINE_NET_WS_CLOSE:
				engine.iEvent.OnWSocketClose(ctx.wsctx.sid)
			case ENGINE_TIMER:
				engine.iEvent.OnTimer(ctx.timerctx.id1, ctx.timerctx.id2, ctx.timerctx.id3, ctx.timerctx.id4, ctx.timerctx.id5, ctx.timerctx.para1, ctx.timerctx.para2)
			}
		case <-engine.stop:
			break LOOP
		}
	}
	engine.release <- struct{}{}
}
