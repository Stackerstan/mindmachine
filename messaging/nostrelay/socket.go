package nostrelay

import (
	"sync"

	"github.com/gorilla/websocket"
)

type WebSocket struct {
	conn  *websocket.Conn
	mutex sync.Mutex
}

func (ws *WebSocket) WriteJSON(any interface{}) error {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()
	return ws.conn.WriteJSON(any)
}

func (ws *WebSocket) WriteMessage(t int, b []byte) error {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()
	return ws.conn.WriteMessage(t, b)
}
