package websocket

import (
	"github.com/gorilla/websocket"
)

func (ws *WebSocket) RegisterDriverConn(driverID string, conn *websocket.Conn) {
	ws.driverConns.Store(driverID, conn)
}

func (ws *WebSocket) GetDriverConn(driverID string) (*websocket.Conn, bool) {
	val, ok := ws.driverConns.Load(driverID)
	if !ok {
		return nil, false
	}

	return val.(*websocket.Conn), true
}

func (ws *WebSocket) Send(conn *websocket.Conn, payload []byte) error {
	return ws.wsWriteMessage(conn, websocket.TextMessage, payload)
}
