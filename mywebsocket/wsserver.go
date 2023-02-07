package mywebsocket

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/holynull/go-log"
)

var Logger = log.Logger("mywebsocket")

const (
	TYPE_PING = "ping"
	TYPE_PONG = "pong"
)

type BaseMessage struct {
	Type     string    `json:"type"`
	CreateAt time.Time `json:"createAt"`
}

var upGrader = &websocket.Upgrader{
	ReadBufferSize:  4096, //指定读缓存区大小
	WriteBufferSize: 1024, // 指定写缓存区大小
	// 检测请求来源
	CheckOrigin: func(r *http.Request) bool {
		if r.Method != "GET" {
			fmt.Println("method is not GET")
			return false
		}
		if r.URL.Path != "/ws" {
			fmt.Println("path error")
			return false
		}
		return true
	},
}

func HandlerConnectReq(c *gin.Context) {
	// todo: 参数应该传递一个设备信息，并用用户的私钥签名
	// 然后，验证用户签名，通过才能连接
	//升级get请求为webSocket协议
	conn, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		Logger.Error(err)
		return
	}
	defer conn.Close()
	for {
		//读取ws中的数据
		mt, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var msg BaseMessage
		err = json.Unmarshal(message, &msg)
		if err != nil {
			Logger.Error(err)
			return
		}
		switch msg.Type {
		case TYPE_PING:
			err := HandlePingMessage(mt, msg, conn)
			if err != nil {
				Logger.Error(err)
			}
		}
	}
}

func HandlePingMessage(mt int, msg BaseMessage, conn *websocket.Conn) error {
	if msg.Type == TYPE_PING {
		poneMsg := BaseMessage{
			Type:     TYPE_PONG,
			CreateAt: time.Now(),
		}
		b, err := json.Marshal(poneMsg)
		if err != nil {
			return err
		}
		//写入ws数据
		err = conn.WriteMessage(mt, b)
		if err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("NOT_A_PINT_PONG_MESSAGE")
	}
}
