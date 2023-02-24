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
	"github.com/holynull/tss-wasm-lib/crypto/dlnproof"
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
		dlnProof1 := dlnproof.NewDLNProof(prime.H1i, prime.H2i, prime.Alpha, prime.P, prime.Q, prime.NTildei)
		dlnProof2 := dlnproof.NewDLNProof(prime.H2i, prime.H1i, prime.Beta, prime.P, prime.Q, prime.NTildei)
		alpha1 := make([][]byte, 0)
		T1 := make([][]byte, 0)
		for i := range dlnProof1.Alpha {
			alpha1 = append(alpha1[:], dlnProof1.Alpha[i].Bytes())
			T1 = append(T1[:], dlnProof1.T[i].Bytes())
		}
		alpha2 := make([][]byte, 0)
		T2 := make([][]byte, 0)
		for i := range dlnProof2.Alpha {
			alpha2 = append(alpha2[:], dlnProof2.Alpha[i].Bytes())
			T2 = append(T2[:], dlnProof2.T[i].Bytes())
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
			Dlnproof1: &DLNProof{
				Alpha: alpha1,
				T:     T1,
			},
			Dlnproof2: &DLNProof{
				Alpha: alpha2,
				T:     T2,
			},
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
	GroupConnOfTasks.Store(gid, gConns)
	for i := range parties {
		protoPrimes[i].PartyIds = pparties
		err := writeMessageConn(OpStartDKG, protoPrimes[i], gConns[i])
		if err != nil {
			return err
		}
	}
	cancel()
	return nil
}

func writeMessageConn(op string, msg protoreflect.ProtoMessage, conn *SyncConn) error {
	conn.Lock.Lock()
	defer func() {
		conn.Lock.Unlock()
		Logger.Debugf("Send %s message to %s finished.", op, conn.Id)
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

func writeBytesConn(op string, msg []byte, conn *SyncConn) error {
	conn.Lock.Lock()
	defer func() {
		conn.Lock.Unlock()
		Logger.Debugf("Send %s message to %s finished.", op, conn.Id)
	}()
	// Logger.Debugf("DataB64: %s", dataBase64Str)
	opMsg := Operation{
		Op:   op,
		Data: msg,
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
			err := writeMessageConn(MpcMessage, &msg, conn)
			if err != nil {
				return err
			}
		}
	} else {
		for i := range msg.To {
			if int(msg.To[i].Index) < len(conns) {
				err := writeMessageConn(MpcMessage, &msg, conns[msg.To[i].Index])
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

const (
	THRESHOLD   = 1
	PARTY_COUNT = 3
)

var OLD_PARTY_INDEX = []int{0, 1}

func handleReqSIGN(msg []byte) error {
	gid := RandStr(16)
	var gConns []*SyncConn
	for i := 0; i < THRESHOLD+1; i++ {
		userId := fmt.Sprintf("user%d", i)
		dId := fmt.Sprintf("did%d", i)
		key := fmt.Sprintf("%s_%s", userId, dId)
		if conn, ok := ConnMap.Load(key); !ok {
			return fmt.Errorf("NO_CONN:%s_%s", userId, dId)
		} else {
			sconn := conn.(*SyncConn)
			gConns = append(gConns[:], sconn)
		}
	}
	GroupConnOfTasks.Store(gid, gConns)
	for i := range gConns {
		unSignMsg := &UnSignedMessage{
			Msg:   msg,
			Index: int32(i),
			Gid:   gid,
		}
		writeMessageConn(OpStartSIGN, unSignMsg, gConns[i])
	}
	return nil
}

func handlerReqResharing() error {
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
	var protoPrimes []*LocalPreParams
	for i := 0; i < PARTY_COUNT; i++ {
		prime, err := keygen.GeneratePreParams(2 * time.Minute)
		if err != nil {
			cancel()
			return err
		}
		dlnProof1 := dlnproof.NewDLNProof(prime.H1i, prime.H2i, prime.Alpha, prime.P, prime.Q, prime.NTildei)
		dlnProof2 := dlnproof.NewDLNProof(prime.H2i, prime.H1i, prime.Beta, prime.P, prime.Q, prime.NTildei)
		alpha1 := make([][]byte, 0)
		T1 := make([][]byte, 0)
		for i := range dlnProof1.Alpha {
			alpha1 = append(alpha1[:], dlnProof1.Alpha[i].Bytes())
			T1 = append(T1[:], dlnProof1.T[i].Bytes())
		}
		alpha2 := make([][]byte, 0)
		T2 := make([][]byte, 0)
		for i := range dlnProof2.Alpha {
			alpha2 = append(alpha2[:], dlnProof2.Alpha[i].Bytes())
			T2 = append(T2[:], dlnProof2.T[i].Bytes())
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
			Dlnproof1: &DLNProof{
				Alpha: alpha1,
				T:     T1,
			},
			Dlnproof2: &DLNProof{
				Alpha: alpha2,
				T:     T2,
			},
		}
		protoPrimes = append(protoPrimes[:], protoPrime)
	}
	var gConns []*SyncConn
	for i := 0; i < PARTY_COUNT; i++ {
		userId := fmt.Sprintf("user%d", i)
		dId := fmt.Sprintf("did%d", i)
		key := fmt.Sprintf("%s_%s", userId, dId)
		if conn, ok := ConnMap.Load(key); !ok {
			return fmt.Errorf("NO_CONN:%s_%s", userId, dId)
		} else {
			sconn := conn.(*SyncConn)
			gConns = append(gConns[:], sconn)
		}
	}
	GroupConnOfTasks.Store(gid, gConns)
	for i := range gConns {
		runOldPary := false
		var _oIndex int
		for oi, oIndex := range OLD_PARTY_INDEX {
			if i == oIndex {
				runOldPary = true
				_oIndex = oi
			}
		}
		msg := &ResharingMessage{
			Index:       int32(i),
			OIndex:      int32(_oIndex),
			RunOldParty: runOldPary,
			Gid:         gid,
			PreParams:   protoPrimes[i],
		}
		writeMessageConn(OpStartRESHARING, msg, gConns[i])
	}
	cancel()
	return nil
}
