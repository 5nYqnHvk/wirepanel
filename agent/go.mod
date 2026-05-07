module github.com/wirepanel/wirepanel/agent

go 1.25.0

require (
	github.com/coder/websocket v1.8.12
	github.com/creack/pty v1.1.24
	github.com/wirepanel/wirepanel/shared v0.0.0
)

replace github.com/wirepanel/wirepanel/shared => ../shared
