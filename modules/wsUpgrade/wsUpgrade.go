package wsUpgrade

import (
	"bytes"
	"errors"
	"github.com/gorilla/websocket"
	"net/http"
)

func checkOrigin(r *http.Request) bool {
	if r.Method != "GET" {
		return false
	}
	return true
}

func Upgrade(w http.ResponseWriter, r *http.Request, readBufferSize int, writeBufferSize int) (*websocket.Conn, error) {
	upgrade := websocket.Upgrader{
		ReadBufferSize:  readBufferSize,
		WriteBufferSize: writeBufferSize,
		CheckOrigin:     checkOrigin,
	}
	//判断请求是否为websocket升级请求。
	if websocket.IsWebSocketUpgrade(r) {
		conn, err := upgrade.Upgrade(w, r, w.Header())
		if err != nil {
			return nil, err
		}
		return conn, nil
	}
	msg := "非websocket升级请求"
	_, _ = w.Write(bytes.NewBufferString(msg).Bytes())
	w.WriteHeader(http.StatusBadRequest)
	return nil, errors.New(msg)
}
