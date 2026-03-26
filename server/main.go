package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

const (
	watchFreq = 500 * time.Millisecond
)

var (
	addr      = flag.String("a", "localhost:8080", "http service address")
	file      = flag.String("f", "", "file to watch and serve")
	upgrader  = websocket.Upgrader{}
	watchChan = make(chan bool)
)

func watchFile() {
	log.Print("watching: " + *file)
	lastStat, err := os.Stat(*file)
	if err != nil {
		log.Print(err.Error())
		return
	}
	for {
		time.Sleep(watchFreq)
		stat, err := os.Stat(*file)
		if err != nil {
			log.Print(err.Error())
		}
		if stat.Size() != lastStat.Size() || stat.ModTime() != lastStat.ModTime() {
			lastStat = stat
			log.Print("file changed")
			watchChan <- true
		}
	}
}

// Websocket
func watch(w http.ResponseWriter, r *http.Request) {
	log.Print("new websocket connection")
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for range watchChan {
		log.Print("sending wensocket notification")
		err = c.WriteMessage(websocket.TextMessage, []byte("update"))
		if err != nil {
			log.Println("write:", err)
			break
		}
	}
}

func server() {
	http.HandleFunc("/watch", watch)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, *file) })
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func main() {
	flag.Parse()
	if *file == "" {
		log.Print("missing file")
		return
	}
	log.Print("starting hotremote server")
	go watchFile()
	server()
}
