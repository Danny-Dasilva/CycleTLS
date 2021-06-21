package main

import (
	"log"
	"net/http"
)

func WsHandle(w http.ResponseWriter, r *http.Request) {

	ws, err := New(w, r)
	if err != nil {
		log.Println(err)
		return
	}

	err = ws.Handshake()
	if err != nil {
		log.Println(err)
		return
	}

	defer ws.Close()

	for {
		frame, err := ws.Recv()
		if err != nil {
			log.Println("Error Decoding", err)
			return
		}

		switch frame.Opcode {
		case 8: // Close
			return
		case 9: // Ping
			frame.Opcode = 10
			fallthrough
		case 0: // Continuation
			fallthrough
		case 1: // Text
			fallthrough
		case 2: // Binary
			if err = ws.Send(frame); err != nil {
				log.Println("Error sending", err)
				return
			}
		}
	}

}
func main() {
	http.HandleFunc("/", WsHandle)
	log.Fatal(http.ListenAndServe(":9001", nil))
}