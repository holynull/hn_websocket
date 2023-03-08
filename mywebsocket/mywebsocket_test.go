package mywebsocket

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/bnb-chain/tss-lib/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/tss"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
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

func TestDataMashall(t *testing.T) {
	gid := RandStr(16)
	parties := tss.GenerateTestPartyIDs(3, 0)
	var pparties []*ProtoPartyID
	var protoPrimes []*LocalPreParams
	for i := range parties {
		var pp = &ProtoPartyID{
			Id:      parties[i].Id,
			Moniker: parties[i].Moniker,
			Key:     parties[i].Key,
			Index:   int32(parties[i].Index),
		}
		pparties = append(pparties[:], pp)
		prime, err := keygen.GeneratePreParams(2 * time.Minute)
		if err != nil {
			t.Error(err)
		}
		protoPrime := &LocalPreParams{
			PaillierSK: &PrivateKey{
				PublicKey: &PublicKey{
					N: prime.PaillierSK.PublicKey.N.Bytes(),
				},
				LambdaN: prime.PaillierSK.LambdaN.Bytes(),
				PhiN:    prime.PaillierSK.PhiN.Bytes(),
			},
			NTildei: prime.NTildei.Bytes(),
			H1I:     prime.H1i.Bytes(),
			H2I:     prime.H2i.Bytes(),
			Alpha:   prime.Alpha.Bytes(),
			Index:   int32(i),
			Gid:     gid,
		}
		protoPrimes = append(protoPrimes, protoPrime)
	}
	var dB64Strings []string
	for i := range parties {
		protoPrimes[i].PartyIds = pparties
		b, err := proto.Marshal(protoPrimes[i])
		if err != nil {
			t.Error(err)
		}
		// Logger.Debugf("DataB64: %s", dataBase64Str)
		opMsg := Operation{
			Op:   OpStartDKG,
			Data: b,
		}
		opJsong, err := json.Marshal(&opMsg)
		if err != nil {
			t.Error(err)
		}
		t.Logf("Server Operation [%d]: %s", i, string(opJsong))
		d, err := proto.Marshal(&opMsg)
		if err != nil {
			t.Error(err)
		}
		b64Str := base64.StdEncoding.EncodeToString(d)
		// dB64, err := EncodeBase64Str(d)
		// if err != nil {
		// 	t.Error(err)
		// }
		dB64Strings = append(dB64Strings[:], b64Str)
	}
	for i := range dB64Strings {
		b, err := base64.StdEncoding.DecodeString(dB64Strings[i])
		// b, err := DecodeBase64Str(dB64Strings[i])
		if err != nil {
			t.Error(err)
			return
		}
		var m Operation
		err = proto.Unmarshal(b, &m)
		if err != nil {
			t.Error(err)
		}
		opJson, err := json.Marshal(&m)
		if err != nil {
			t.Error(err)
		}
		t.Logf("Client Operation [%d]: \n %s", i, string(opJson))
		var preParam LocalPreParams
		err = proto.Unmarshal(m.Data, &preParam)
		if err != nil {
			t.Error(err)
		}
		jb, err := json.Marshal(&preParam)
		if err != nil {
			t.Error(err)
		}
		t.Logf("PreParam[%d]: \n %s", i, string(jb))
	}
}

func TestAaaa(t *testing.T) {
	s := "CglzdGFydF9ka2cSzBRDb3dHQ29NQ0NvQUN6b2xEWnlhZjFFdTFyM2hxOW1MOFBESzA4NUgzcndUZE5xeVkyTHh2YWJMTHlDTnlCRGVKcVhZRGhoOFk2cUZ1NnBPM2FuSUhHZTEwd0NNdkhuanMyNjdPaU9LTXpiUkc0cGRJSEFkMjU0cTRQU1p3dzlBMEpiWjk1OVNYSklBWHJBUU9KWFhxcVgrYm1QdTl5QjFPRVpyZ3Y5bUM0cjJ1Z01sOXlQQVBpaU9sM2o0L0VKRmxRNzU3aGc0VVd4Tm5MaGhHM3dGZkZHcHNHM09Fa1VTR1MycXBPczNBM0JPUFdqQnFLOUVSOEI0NWVCcDFsK3NodVNTZjNxNGE2NGxxQm80TXJiZEswcUtYYWxqdEJyN2hRZjZLWTF1V0NRbnN4azJaVjRHWE8vK3JDL3lkbExLQmNxV3VaMExCdzhOUTh3L0MwOUgxV1AvaU4wdjQzNlVtRlJLQUFtZEVvYk9UVCtvbDJ0ZThOWHN4Zmg0WldubkkrOWVDYnB0V1RHeGVON1RaWmVRUnVRSWJ4TlM3QWNNUGpIVlF0M1ZKMjdVNUE0ejJ1bUFSbDQ4OGRtM1haMFJ4Um1iYUkzRkxwQTREdTNQRlhCNlRPR0hvR2hMYlB2UHFTNUpBQzlZQ0J4SzY5VlMvemN4OTN1UU9wd2pOY0Yvc3dYRmUxMEJrdnVSNEI4VVE3RU4wOVBVT2xsRm9WNTFqTUgzUzd5MnNRNjVOU0RreVlIMldZT1pUajlPbUFPTG14NUNSdlVPRmVZMnl4RVZNUllOZHB5S0RxSi9ncVFIV1JGaHBYUGVNUUEvRGwzYUszM09vZXNVVVhKMC9Oa09hdHAyejlCajArQTJ2SU0xdmRPRi8rcWZ4MWFVakRUZG1HWnltS2FqMUZ2bWpzS2pmYlhCQldWdFNSZ1lhZ0FMT2lVTm5KcC9VUzdXdmVHcjJZdnc4TXJUemtmZXZCTjAyckpqWXZHOXBzc3ZJSTNJRU40bXBkZ09HSHhqcW9XN3FrN2RxY2djWjdYVEFJeThlZU96YnJzNkk0b3pOdEViaWwwZ2NCM2JuaXJnOUpuREQwRFFsdG4zbjFKY2tnQmVzQkE0bGRlcXBmNXVZKzczSUhVNFJtdUMvMllMaXZhNkF5WDNJOEErS0lkaUc2ZW5xSFN5aTBLODZ4bUQ3cGQ1YldJZGNtcEJ5Wk1EN0xNSE1weCtuVEFIRnpZOGhJM3FIQ3ZNYlpZaUttSXNHdTA1RkIxRS93VklEcklpdzBybnZHSUFmaHk3dEZiN25VUFdLS0xrNmZteUhOVzA3WitneDZmQWJYa0dhM3VuQy8vVlA0NnRLUmhwdXpETTVURk5SNmkzelIyRlJ2dHJnZ3JLMnBJd01Fb0FDeGFIRkg1ZjcyQlRyYU1UQjJPZm5MWGhTSFRwMkhINWllaklPa1Nwa04va1Q3ZEJ0K2pLZmwvbDl3QlJFb2R0cEEzWEZQRlJZYWZaQTBPTzA3TjRYSi9iNEdlS3BNQU1GWHVoaklHUDc0LzZaNlBKYVNCb2x1d3BwMENNNmdVNG90TUVwdlNqYkJFWmtjdWlSLzBOSzFMdjhza09QcEU3eVpzVjdweWxuWitkaFdiQnUxdHk3TmVqbWIrRGZCYzNXSUhsTlVPQmxjS01Pd1IrYkJUU1V5QktSN2lMem85QUZPMFVEM3hzKzU2M1hHbTNTUmZNcEpsNWlCMU44bmZaS0l4bzJ0OXVMTGcvcGMvWWdZMmRJeEt6TFNJYWxEd0pBbHE3RC96cmVwbzdyODVjSGlrZXdUOS93V2dtWkdvRGJ6RVFzUFNJVHFobXBTVExGelVzUk5ScUFBaGhTb05pZ29EZmdyUU1Fb1RiUVBQRitOTnE5STVZSzNldEk3NFV4S1J3US9hM21xaTJ1OE83cUJqVHo4ajI1S1F4RUZOWVZKcmdkek40THVrVXBHRGU4eExEUjlmNHVrRWIwbVkyNXRINFVseDUvbDNhSWZIWGZsQXZQZFVYMlJEVU1aV3NidWFVTlN2Wm51ekFYMDlqcHVuL0FWU1RjQkhhZ3kzdEYzWm5PdW0rNTlNaUYvYURyVjAxS0syaWVyMHA4a2Z6WnJCZzJCOUs1Q0p3ZmNnRk9RMWh3NVhVMXFIcEREdXplREEwUmg5WndVcFV5V2E4cE1RSkR4ZjlkTEZkNUxGWUY2VC9WZWZic2grQUt1M2Q5Lys5MHgwaEYzbFVqTnF2NXNadDZOZVVMSzRxT2x1VGU2UWVqeWE0T3JBZHRoZUplcFFKUVZlTEFyVXI5SGFzaWdBSXVhSEhlalI0a2s5eFdqREVjY3BIeTFzeGdBWWVIVkxUT2d5N3VEU3J0SnA5S2hDL1FBYk9sY3grd29yWEVKdmZpeDZ1ZlFUNk84R3Z5TE9nV1BVWXkxamdNR0pDYlhvWmptcGQ0ZW9TdU1vdjJVL2NXaEFvRUtleXRud0dYSFIrZzZBOFRzMFdYSTBHTGxiTVJiTytPeVphWVZOSFlSLzBqclRNOGdZZ3R6YzBTVnpxVjAxcWF1OFZMeDNYNVhLR01yakVSNklRK0NJRnhCaFVCSnplemx5ZFgxTUg0amNDYlVaSVp5V0RBeWV3dTJiRFBYWnI2aXZqY0VOWmNZTFlKZnpNdk1ydTM0VTJJL2FUVmgrRVRaK3NYUENWc3J4ZU5uMitTTDFzMFdKbHE4VmprOGU2Q2pBZ0puL1QzSHFnUW5HV3E2QzRUa2VUMFZWbXZGTk9BS29BQ0VhZDQreGJlSDNwTUJyWlZGVE9kZm9FU25HcTR0VkE2djN2ZHp0ZkF1UVd1S1hiYXBCU05jQm9jc0NyL0poMjJZM2ozUjlyaENZYVE2MUtlcnVwbEJQUktRd0JqZnNsNnExTG5lUzIxM2toZnNHNnhqWm80ZUtBR0FKVjA1ZzJISlFjRlNvcHlWc1QxQnJTNlBqVWFNdm1HUStXU1U2cm5HdlR1b3drNy9BN2cyQlA3VXhEb2FiRnI2THN2eklhV2x1Yk04YnlFWDhPaWU1Q1ZGUnUyYnkrajhZYWYxNHUyMS9SNnZxVW8wNmZnSmcvU0FDUnkydEtOWFY1ZFJlOWJQY21wU3FuRFpBbW80azJmNjJaVkNSWm1zejQ3ajAvQnVmV3lCQ0o3Z1psdlhMWTNkREduNkR1NzNSUmZ3MzZDMFhzYVdPVkN0a3d6RzJuQkUxNkhRRW9yQ2dFeEVnUlFXekZkR2lDZ1hOTzZjNzlLTXBQdzVQV005MHp6UklRdkhrT0R2VkFVOGp5M3o5WE9nRW90Q2dFeUVnUlFXekpkR2lDZ1hOTzZjNzlLTXBQdzVQV005MHp6UklRdkhrT0R2VkFVOGp5M3o5WE9nU0FCU2kwS0FUTVNCRkJiTTEwYUlLQmMwN3B6djBveWsvRGs5WXozVFBORWhDOGVRNE85VUJUeVBMZlAxYzZDSUFKUUFWb1FiRXhWZUd0eVltOTRjbFpIWlRS"
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Error(err)
	}
	var op Operation
	err = proto.Unmarshal(b, &op)
	if err != nil {
		t.Error(err)
	}
}
