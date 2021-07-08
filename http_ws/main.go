package main

import (
	"log"
	"net/url"

	"github.com/gorilla/websocket"
)

func main() {
    u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
    conn, _, err := DefaultDialer.Dial(u.String(), nil)
    if err != nil {
        log.Fatalf("dial err: %v", err)
    }
    err = conn.WriteMessage(websocket.TextMessage, []byte("hellow websockets"))
    if err != nil {
        log.Fatalf("msg err: %v", err)
    }
}
