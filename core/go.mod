module github.com/wirepanel/wirepanel/core

go 1.25.0

require (
	github.com/coder/websocket v1.8.12
	github.com/google/uuid v1.6.0
	github.com/wirepanel/wirepanel-pro v0.0.0
	github.com/wirepanel/wirepanel/shared v0.0.0
	golang.org/x/crypto v0.50.0
)

replace (
	github.com/wirepanel/wirepanel-pro => ../pro
	github.com/wirepanel/wirepanel/shared => ../shared
)
