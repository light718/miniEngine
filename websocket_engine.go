package miniEngine

import (
	"log"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

const (
	//无效会话ID
	INVALID_SID int64 = 0
)

type Message struct {
	Sid     int64
	Content []byte
}

// 会话
type wsConnection struct {
	sid   int64
	conn  *websocket.Conn
	send  chan []byte
	close chan *sync.Pool
}

func NewWebSocketConnection() (c *wsConnection) {
	c = &wsConnection{
		sid:   INVALID_SID,
		send:  make(chan []byte, 16),
		close: make(chan *sync.Pool),
	}

	return
}

func (c *wsConnection) run() {
	var pool *sync.Pool = nil
LOOP:
	for {
		select {
		case content := <-c.send:
			c.writeMessage(content)
		case p := <-c.close:
			{
				pool = p
				c.conn.Close()
				c.conn = nil
				break LOOP
			}

		}
	}
	if pool != nil {
		pool.Put(c)
	}
}

func (c *wsConnection) Close(pool *sync.Pool) {
	c.close <- pool
}

func (c *wsConnection) writeMessage(content []byte) {
	if c.conn == nil {
		return
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, content); err != nil {
		log.Panicf("write message painc error:%v", err)
		return
	}
}

type WebSocketEngine struct {
	httpsrv         *http.Server
	upgrader        websocket.Upgrader //升级处理
	iAttemperEngine *AttemperhEngine   //接口服务
	identity        int64              //ID
	message         chan Message
	register        chan *wsConnection
	unregister      chan int64
	pool            sync.Pool
	clients         map[int64]*wsConnection
	breakloop       chan struct{}
	release         chan struct{}
}

func NewWebSocketEngine(addr string, p *AttemperhEngine, cache int) (engine *WebSocketEngine) {
	engine = &WebSocketEngine{
		httpsrv:         &http.Server{Addr: addr, Handler: nil},
		iAttemperEngine: p,
		identity:        INVALID_SID,
		clients:         make(map[int64]*wsConnection),
		message:         make(chan Message, cache),
		register:        make(chan *wsConnection, 1024),
		unregister:      make(chan int64, 1024),
		breakloop:       make(chan struct{}),
		release:         make(chan struct{}),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		pool: sync.Pool{
			New: func() any {
				return NewWebSocketConnection()
			},
		},
	}
	http.HandleFunc("/ws", engine.Handler)
	return
}

func (engine *WebSocketEngine) Dispatch() {
	defer func() {
		if err := recover(); err != nil {
			log.Fatalf("WebSocketEngine Dispatch error:%v", err)
		}
	}()
LOOP:
	for {
		select {
		case conn := <-engine.register:
			engine.clients[conn.sid] = conn
		case sid := <-engine.unregister:
			if conn, ok := engine.clients[sid]; ok {
				conn.Close(&engine.pool)
			}
			delete(engine.clients, sid)
		case msg := <-engine.message:
			if conn, ok := engine.clients[msg.Sid]; ok {
				conn.send <- msg.Content
			}
		case <-engine.breakloop:
			engine.httpsrv.Close()
			for _, v := range engine.clients {
				v.Close(&engine.pool)
			}
			break LOOP
		}
	}
	engine.release <- struct{}{}
}

func (engine *WebSocketEngine) start() {
	//启动http服务
	go engine.httpsrv.ListenAndServe()
	go engine.Dispatch()
}

// 停止
func (engine *WebSocketEngine) stop() {
	engine.breakloop <- struct{}{}
	<-engine.release
}

// 连接处理
func (engine *WebSocketEngine) Handler(w http.ResponseWriter, r *http.Request) {
	c, err := engine.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close()
	connection := engine.pool.Get().(*wsConnection)
	connection.sid = atomic.AddInt64(&engine.identity, 1)
	connection.conn = c
	go connection.run()
	engine.register <- connection
	engine.iAttemperEngine.doWebSocketConn(connection.sid, r.RemoteAddr)
	engine.handleReads(connection)
	engine.iAttemperEngine.doWebSocketClose(connection.sid)
	engine.unregister <- connection.sid
}

func (engine *WebSocketEngine) handleReads(connection *wsConnection) {
	for {
		messageType, message, err := connection.conn.ReadMessage()
		if err != nil {
			break
		}
		if messageType == websocket.TextMessage {
			engine.iAttemperEngine.doWebSocketRecv(connection.sid, message)
		}
	}
}

// 发送接口
func (engine *WebSocketEngine) Send(sid int64, content []byte) {
	engine.message <- Message{Sid: sid, Content: content}
}

// 关闭会话
func (engine *WebSocketEngine) Close(sid int64) {
	engine.unregister <- sid
}
