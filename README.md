# Hot Remote

Hot-reload on a remote host.

This is a very simple tool that allows you to “hot reload”, meaning restart on a
change, from a remote host.

Basically the server will run on the development machine and expose 2 things, an
API that will serve a single file, and a websocket that will send a notification
if that file change. The client will connect to that websocket, and download the
file when it changes (when notified by the websocket) and run a command once the
download is complete.

There is nothing more to it.

## Server

```bash
cd server
go run . -a 192.168.56.1:7070 -f test.exe
```

## Client

Example client for Windows.

Build client:

```bash
cd client
GOOS=windows go build -o hotremote.exe
```

Run client:

```bash
hotremote.exe -a 192.168.56.1:7070 -f test.exe -c 'cmd.exe /c test.exe' -d 3s
```

## License

MIT
