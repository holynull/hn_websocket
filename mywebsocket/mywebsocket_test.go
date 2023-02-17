package mywebsocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestStartWsServer(t *testing.T) {
	bindAddress := fmt.Sprintf("%s:%d", "localhost", 17790)
	r := gin.Default()
	r.GET("/ws", HandlerConnectReq)
	r.Run(bindAddress)
}

func TestWsClient(t *testing.T) {
	//创建一个拨号器，也可以用默认的 websocket.DefaultDialer
	dialer := websocket.Dialer{}
	//向服务器发送连接请求，websocket 统一使用 ws://，默认端口和http一样都是80
	connect, _, err := dialer.Dial("ws://127.0.0.1:17790/ws", nil)
	if nil != err {
		t.Error(err)
		return
	}
	//离开作用域关闭连接，go 的常规操作
	defer connect.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//定时向客户端发送数据
	go tickWriter(ctx, connect)

	go func() {
		//启动数据读取循环，读取客户端发送来的数据
		for {
			select {
			case <-ctx.Done():
				break
			default:
				//从 websocket 中读取数据
				//messageType 消息类型，websocket 标准
				//messageData 消息数据
				messageType, messageData, err := connect.ReadMessage()
				if nil != err {
					log.Println(err)
					break
				}
				switch messageType {
				case websocket.TextMessage: //文本数据
					t.Log(string(messageData))
				case websocket.BinaryMessage: //二进制数据
					t.Log(messageData)
				case websocket.CloseMessage: //关闭
				case websocket.PingMessage: //Ping
				case websocket.PongMessage: //Pong
				default:

				}
			}
		}
	}()
	time.Sleep(13 * time.Second)
	cancel()
}
func tickWriter(ctx context.Context, connect *websocket.Conn) {
	for {
		select {
		case <-ctx.Done():
			break
		default:
			pingMsg := BaseMessage{
				Type:     TypePing,
				CreateAt: time.Now(),
			}
			b, err := json.Marshal(pingMsg)
			if err != nil {
				panic(err)
			}
			//向客户端发送类型为文本的数据
			err = connect.WriteMessage(websocket.TextMessage, b)
			if nil != err {
				panic(err)
			}
			//休息一秒
			time.Sleep(time.Second)
		}
	}
}
