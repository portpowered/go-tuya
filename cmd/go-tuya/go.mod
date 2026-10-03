module github.com/portpowered/go-tuya/cmd/go-tuya

go 1.24.0

require (
	github.com/eclipse/paho.mqtt.golang v1.5.1
	github.com/portpowered/go-tuya v0.3.5
	github.com/yeqown/go-qrcode/v2 v2.2.5
	github.com/yeqown/go-qrcode/writer/terminal v1.1.2
	golang.org/x/sys v0.40.0
	golang.org/x/term v0.39.0
)

replace github.com/portpowered/go-tuya => ../..

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/nsf/termbox-go v1.1.1 // indirect
	github.com/yeqown/reedsolomon v1.0.0 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
)
