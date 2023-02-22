package mywebsocket

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/holynull/go-log"
	"google.golang.org/protobuf/proto"
)

var Logger = log.Logger("mywebsocket")

const (
	TypePing      = "ping"
	TypePong      = "pong"
	OpReqDKG      = "reqdkg"
	OpStartDKG    = "start_dkg"
	MpcDKGMessage = "mpc_dkg_message"
)

type BaseMessage struct {
	Type     string    `json:"type"`
	CreateAt time.Time `json:"createAt"`
}

var upGrader = &websocket.Upgrader{
	// ReadBufferSize:  4096, //指定读缓存区大小
	// WriteBufferSize: 1024, // 指定写缓存区大小
	// // 检测请求来源
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

var ConnMap sync.Map

type SyncConn struct {
	Id   string
	Conn *websocket.Conn
	Lock sync.Mutex
}

func lenSyncMap(m *sync.Map) int {
	var i int
	m.Range(func(k, v interface{}) bool {
		i++
		return true
	})
	return i
}

func HandlerConnectReq(c *gin.Context) {
	userId, ok := c.GetQuery("userId")
	if !ok {
		c.JSON(http.StatusBadRequest, "userId required.")
		return
	}
	dId, ok := c.GetQuery("dId")
	if !ok {
		c.JSON(http.StatusBadRequest, "dId required.")
		return
	}
	key := fmt.Sprintf("%s_%s", userId, dId)
	//升级get请求为webSocket协议
	conn, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		Logger.Error(err)
		return
	}
	sconn := &SyncConn{
		Id:   key,
		Conn: conn,
	}
	ConnMap.Store(key, sconn)
	defer conn.Close()
	for {
		Logger.Debug("Reading data from conn.")
		//读取ws中的数据
		mt, message, err := conn.ReadMessage()
		if err != nil {
			Logger.Error(err)
			ConnMap.Delete(key)
			break
		}
		Logger.Debug("Handle messages read.")
		switch mt {
		case websocket.TextMessage:
			b, err := base64.StdEncoding.DecodeString(string(message))
			if err != nil {
				Logger.Error(err)
				ConnMap.Delete(key)
				return
			}
			var pMsg Operation
			proto.Unmarshal(b, &pMsg)
			switch pMsg.Op {
			case TypePing:
				Logger.Debugf("Get a message: %s", message)
			// err := HandlePingMessage(mt, msg, conn)
			// if err != nil {
			// 	ConnMap.Delete(key)
			// 	Logger.Error(err)
			// }
			case OpReqDKG:
				Logger.Debug("Get a message: OP_REQ_DKG")
				err := handleReqDKG()
				if err != nil {
					Logger.Error(err)
				}
				Logger.Debug("Send OpStartDKG to client.")
			case MpcDKGMessage:
				Logger.Debug("Get a message: MpcDKGMessage")
				err := handleMpcDKGMessage(pMsg.Data)
				if err != nil {
					Logger.Error(err)
				}
			default:
				Logger.Infof("Message is: %s", message)
			}
		case websocket.BinaryMessage:
			Logger.Infof("Message type is BinaryMessage, %d", mt)
		default:
			Logger.Infof("Message type: %d", mt)
		}
	}
}

func HandlePingMessage(mt int, msg BaseMessage, conn *websocket.Conn) error {
	if msg.Type == TypePing {
		poneMsg := BaseMessage{
			Type:     TypePong,
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
