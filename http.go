package main

import (
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
var clients = make(map[*websocket.Conn]bool)

func wsHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}

	// register client
	clients[ws] = true
}

func serveHTTP(dataChan chan []AudioStats, addr string) {
	router := mux.NewRouter()
	files := http.FileServer(http.Dir("static"))
	router.Handle("/", files)
	router.HandleFunc("/ws/live", wsHandler)

	go func() {
		for {
			select {
			case data := <-dataChan:
				for client := range clients {
					err := client.WriteJSON(data)
					if err != nil {
						log.Printf("Websocket error %s", err)
						client.Close()
						delete(clients, client)
					}
				}
			}
		}
	}()

	log.Fatal(http.ListenAndServe(addr, router))
}
