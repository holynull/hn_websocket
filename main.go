package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/holynull/hn_websocket/mywebsocket"
)

func main() {
	bindAddress := fmt.Sprintf("%s:%d", "localhost", 17790)
	r := gin.Default()
	r.GET("/ws", mywebsocket.HandlerConnectReq)
	r.Run(bindAddress)
}
