package mywebsocket

import (
	"bytes"
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
	"google.golang.org/protobuf/reflect/protoreflect"
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
	gid := RandStr(16)
	connLen := lenSyncMap(&ConnMap)
	if connLen != 3 {
		return errors.New("LEN_OF_DEVICES_CONN_LESS_THAN_3")
	}
	parties := tss.GenerateTestPartyIDs(3, 0)
	var partiesb [][]byte
	var primes []*keygen.LocalPreParams
	var protoPrimes []*LocalPreParams
	var gConns []*websocket.Conn
	for i, p := range parties {
		var pp = &ProtoPartyID{
			Id:      p.Id,
			Moniker: p.Moniker,
			Key:     p.Key,
			Index:   int32(p.Index),
		}
		b, err := proto.Marshal(pp)
		if err != nil {
			return err
		}
		partiesb = append(partiesb[:], b)
		prime, err := keygen.GeneratePreParams(2 * time.Minute)
		if err != nil {
			return err
		}
		primes = append(primes[:], prime)
		protoPrime := &LocalPreParams{
			PaillierSK: &PrivateKey{
				PublicKey: &PublicKey{
					N: prime.PaillierSK.PublicKey.N.Bytes(),
				},
				LambdaN: prime.PaillierSK.LambdaN.Bytes(),
				PhiN:    prime.PaillierSK.PhiN.Bytes(),
			},
			NTildei:  prime.NTildei.Bytes(),
			H1I:      prime.H1i.Bytes(),
			H2I:      prime.H2i.Bytes(),
			Alpha:    prime.Alpha.Bytes(),
			PartyIds: partiesb,
			Index:    int32(i),
			Gid:      gid,
		}
		protoPrimes = append(protoPrimes, protoPrime)

		userId := fmt.Sprintf("user%d", i)
		dId := fmt.Sprintf("did%d", i)
		if conn, ok := ConnMap.Load(fmt.Sprintf("%s_%s", userId, dId)); !ok {
			return fmt.Errorf("NO_CONN:%s_%s", userId, dId)
		} else {
			gConns = append(gConns[:], conn.(*websocket.Conn))
		}
	}
	GroupConnOfTasks.Store(gid, gConns)
	for i := range parties {
		err := writeMessageToConn(OpStartDKG, protoPrimes[i], gConns[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func writeMessageToConn(op string, msg protoreflect.ProtoMessage, conn *websocket.Conn) error {
	b, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	opMsg := Operation{
		Op:   op,
		Data: b,
	}
	d, err := proto.Marshal(&opMsg)
	if err != nil {
		return err
	}
	encodedData := &bytes.Buffer{}
	encodeer := base64.NewEncoder(base64.StdEncoding, encodedData)
	_, err = encodeer.Write(d)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, encodedData.Bytes())
}

func handleMpcDKGMessage(data []byte) error {
	var msg ProtoMpcMessage
	err := proto.Unmarshal(data, &msg)
	if err != nil {
		return err
	}
	toIndex := msg.To.Index
	val, ok := GroupConnOfTasks.Load(msg.Gid)
	if !ok {
		return fmt.Errorf("CAN_NOT_FIND_GROUP: %s", msg.Gid)
	}
	conns := val.([]*websocket.Conn)
	writeMessageToConn(MpcDKGMessage, &msg, conns[toIndex])
	return nil
}
