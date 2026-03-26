package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"github.com/google/shlex"
	"github.com/gorilla/websocket"
)

var (
	addr      = flag.String("a", "localhost:8080", "http service address")
	file      = flag.String("f", "", "file name to save on disk")
	command   = flag.String("c", "", "command to execute on file update")
	delay     = flag.Duration("d", 3*time.Second, "delay between reconnection attempts")
	watchChan = make(chan bool)
	interrupt = make(chan os.Signal, 1)
)

func downloadFile() (err error) {
	log.Print("downloading file")

	out, err := os.Create(*file)
	if err != nil {
		return err
	}
	defer out.Close()

	u := url.URL{Scheme: "http", Host: *addr, Path: "/"}
	resp, err := http.Get(u.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func run(ctx context.Context, program string, args []string) {
	log.Print("starting command")

	cmd := exec.CommandContext(ctx, program, args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		fmt.Print("error running command: " + err.Error())
	}
	err = cmd.Wait()
	if err != nil {
		log.Println("waiting on cmd:", err)
	}
}

func watch(scommand string, sargs []string) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		run(ctx, scommand, sargs)
	}()

	for range watchChan {
		cancel()
		ctx, cancel = context.WithCancel(context.Background())
		downloadFile()
		go func() {
			run(ctx, scommand, sargs)
		}()
	}

	cancel()
}

// connectAndListen dials the WebSocket, reads messages, and forwards
// file-change notifications to watchChan. It returns true if the caller should
// reconnect (connection lost), or false if the user interrupted.
func connectAndListen(wsURL string, firstConnect bool) bool {
	log.Printf("connecting to %s", wsURL)

	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Println("dial:", err)
		return true
	}
	defer c.Close()

	if !firstConnect {
		log.Print("reconnected, downloading latest file")
		if err := downloadFile(); err != nil {
			log.Println("download after reconnect:", err)
		} else {
			watchChan <- true
		}
	}

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %s", message)
			if string(message) == "update" {
				log.Print("file changed")
				watchChan <- true
			}
		}
	}()

	for {
		select {
		case <-done:
			// connection lost (server shutdown, network drop, etc.)
			return true
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return false
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return false
		}
	}
}

func main() {
	flag.Parse()
	log.SetFlags(0)

	if *file == "" {
		log.Print("missing file")
		return
	}

	downloadFile()

	if *command == "" {
		log.Print("missing command")
		return
	}

	s, err := shlex.Split(*command)
	if err != nil {
		log.Print(err.Error())
		return
	}

	go watch(s[0], s[1:])

	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/watch"}
	wsURL := u.String()

	firstConnect := true
	for {
		reconnect := connectAndListen(wsURL, firstConnect)
		if !reconnect {
			return
		}
		firstConnect = false

		log.Printf("reconnecting in %s...", *delay)
		select {
		case <-time.After(*delay):
			// Continue to next reconnection attempt
		case <-interrupt:
			log.Println("interrupt")
			return
		}
	}
}
