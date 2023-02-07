package mywebsocket

import (
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestStartWsServer(t *testing.T) {
	bindAddress := fmt.Sprintf("%s:%d", "localhost", 17790)
	r := gin.Default()
	r.GET("/ws", HandlerConnectReq)
	r.Run(bindAddress)
}
