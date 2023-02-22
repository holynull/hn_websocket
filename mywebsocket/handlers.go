package mywebsocket

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/holynull/tss-wasm-lib/ecdsa/keygen"
	"github.com/holynull/tss-wasm-lib/tss"
	"google.golang.org/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

var GroupConnOfTasks sync.Map

func RandStr(length int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	bytes := []byte(str)
	result := []byte{}
	rand.Seed(time.Now().UnixNano() + int64(rand.Intn(100)))
	for i := 0; i < length; i++ {
		result = append(result, bytes[rand.Intn(len(bytes))])
	}
	return string(result)
}
func handleReqDKG() error {
	ctx, cancel := context.WithCancel(context.Background())
	preTicker := time.NewTicker(time.Second)
	go func() {
		counter := 0
		for {
			select {
			case <-preTicker.C:
				counter++
				Logger.Debugf("Waiting in %d s", counter)
			case <-ctx.Done():
				preTicker.Stop()
				break
			}
		}
	}()
	gid := RandStr(16)
	connLen := lenSyncMap(&ConnMap)
	if connLen != 3 {
		cancel()
		return errors.New("LEN_OF_DEVICES_CONN_LESS_THAN_3")
	}
	parties := tss.GenerateTestPartyIDs(3, 0)
	var pparties []*ProtoPartyID
	var protoPrimes []*LocalPreParams
	var gConns []*SyncConn
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
			cancel()
			return err
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
			Beta:    prime.Beta.Bytes(),
			P:       prime.P.Bytes(),
			Q:       prime.Q.Bytes(),
			Index:   int32(i),
			Gid:     gid,
		}
		protoPrimes = append(protoPrimes, protoPrime)

		userId := fmt.Sprintf("user%d", i)
		dId := fmt.Sprintf("did%d", i)
		if conn, ok := ConnMap.Load(fmt.Sprintf("%s_%s", userId, dId)); !ok {
			cancel()
			return fmt.Errorf("NO_CONN:%s_%s", userId, dId)
		} else {
			gConns = append(gConns[:], conn.(*SyncConn))
		}
	}
	cancel()
	GroupConnOfTasks.Store(gid, gConns)
	for i := range parties {
		protoPrimes[i].PartyIds = pparties
		err := writeMessageConn(OpStartDKG, protoPrimes[i], gConns[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func writeMessageConn(op string, msg protoreflect.ProtoMessage, conn *SyncConn) error {
	conn.Lock.Lock()
	defer func() {
		conn.Lock.Unlock()
		Logger.Debugf("Send %s message finished.", op)
	}()
	b, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	// Logger.Debugf("DataB64: %s", dataBase64Str)
	opMsg := Operation{
		Op:   op,
		Data: b,
	}
	d, err := proto.Marshal(&opMsg)
	if err != nil {
		return err
	}
	dB64 := base64.StdEncoding.EncodeToString(d)
	time.Sleep(20 * time.Millisecond)
	return conn.Conn.WriteMessage(websocket.TextMessage, []byte(dB64))
}

func handleMpcDKGMessage(data []byte) error {
	var msg ProtoMpcMessage
	err := proto.Unmarshal(data, &msg)
	if err != nil {
		return err
	}
	val, ok := GroupConnOfTasks.Load(msg.Gid)
	if !ok {
		return fmt.Errorf("CAN_NOT_FIND_GROUP: %s", msg.Gid)
	}
	conns := val.([]*SyncConn)
	if msg.To == nil || len(msg.To) == 0 {
		for _, conn := range conns {
			err := writeMessageConn(MpcDKGMessage, &msg, conn)
			if err != nil {
				return err
			}
		}
	} else {
		for _, p := range msg.To {
			err := writeMessageConn(MpcDKGMessage, &msg, conns[p.Index])
			if err != nil {
				return err
			}
		}
	}
	return nil
}
