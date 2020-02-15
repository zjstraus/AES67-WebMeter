package main

import (
	"fmt"
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

func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "home")
}

func serveHTTP(dataChan chan []AudioStats) {
	router := mux.NewRouter()
	router.HandleFunc("/", rootHandler).Methods("GET")
	router.HandleFunc("/ws", wsHandler)

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

	log.Fatal(http.ListenAndServe(":8844", router))
}
