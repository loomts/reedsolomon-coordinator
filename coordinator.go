package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/klauspost/reedsolomon"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/multiformats/go-multiaddr"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"strconv"
)

type Node struct {
	Host string  `yaml:"host"`
	Port int     `yaml:"port"`
	ID   peer.ID `yaml:"-"`
}

type RFile struct {
	MD5    string
	name   string
	Size   int64
	RSize  int
	Parity int
	S2P    map[int]int // shard to peer
}

type RC struct {
	Enc          reedsolomon.Encoder
	Ctx          context.Context
	Me           Node
	Host         host.Host
	Peers        []Node `yaml:"nodes"` // peers
	RFile        []RFile
	TranCh       []chan interface{} // Transfer Channel -> transfer status
	TranProtocol protocol.ID
}

func (rc *RC) add(fName string) {
	bfile, err := os.ReadFile(fName)
	handleErr(err)

	shards, err := rc.Enc.Split(bfile)
	handleErr(err)
	err = rc.Enc.Encode(shards)
	handleErr(err)
	fDetail, err := os.Stat(fName)
	handleErr(err)
	rf := RFile{
		MD5:    hex.EncodeToString(md5.New().Sum(bfile)),
		name:   fName,
		Size:   fDetail.Size(),
		RSize:  *dataShard + *parShard,
		Parity: *parShard,
		S2P:    make(map[int]int),
	}
	for i := 0; i < len(shards); i++ {
		fmt.Printf("Shard%d Writing to Peer%d", i, i%len(rc.Peers))
		rf.S2P[i] = i % len(rc.Peers)
		sName := fName + "." + strconv.Itoa(i)
		go rc.sendShard(&shards[i], i%len(rc.Peers), sName)
	}
	rc.RFile = append(rc.RFile, rf)
}

func (rc *RC) sendShard(shard *[]byte, id int, sName string) {
	stream, err := rc.Host.NewStream(rc.Ctx, rc.Peers[id].ID, rc.TranProtocol)
	_, err = stream.Write([]byte(sName + "\n"))
	handleErr(err)
	_, err = io.Copy(stream, bytes.NewReader(*shard))
	handleErr(err)
}

func (rc *RC) handleShard(stream network.Stream) {
	buf := make([]byte, 1024)
	n, err := stream.Read(buf)
	handleErr(err)
	sName := string(buf[:n])
	file, err := os.Create("./reedsolomon-coordinator/" + sName)
	defer file.Close()
	if err != nil {
		fmt.Printf("cannot create file：%s\n", err)
		return
	}
	_, err = io.Copy(file, stream)
	if err != nil {
		fmt.Printf("transfer fail：%s\n", err)
		return
	}
	fmt.Println(sName + "transfer success")
}

func (rc *RC) get(fname string) os.File {
	return os.File{}
}

func (rc *RC) daemon() {
	//node.NewStream()
	//io.Copy()
	if err := rc.Host.Close(); err != nil {
		panic(err)
	}
}

func (rc *RC) ReadConfig() {
	data, err := os.ReadFile("config.yml")
	handleErr(err)
	dataStr := os.ExpandEnv(string(data))
	err = yaml.Unmarshal([]byte(dataStr), &rc)
	fmt.Println(len(rc.Peers))

	handleErr(err)
	os.Mkdir("~/.reedsolomon-coordinator", 0777)
	for i := 0; i < len(rc.Peers); i++ {
		if rc.Peers[i].Port == 0 {
			port, err := FindFreePort(rc.Peers[i].Host, 1024)
			handleErr(err)
			rc.Peers[i].Port = port
		}
	}
	myHost, myIdx := GetLocalIP(), -1
	for i := 0; i < len(rc.Peers); i++ {
		fmt.Printf("trying to connect %s\n", fmt.Sprintf("/ip4/%s/tcp/%d", rc.Peers[i].Host, rc.Peers[i].Port))
		addr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", rc.Peers[i].Host, rc.Peers[i].Port))
		handleErr(err)
		info, err := peer.AddrInfoFromP2pAddr(addr)
		rc.Peers[i].ID = info.ID
		handleErr(err)
		if myHost == rc.Peers[i].Host {
			rc.Me = rc.Peers[i]
			myIdx = i
		} else {
			// persistent peers message
			handleErr(err)
			rc.Host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
		}
	}
	rc.Host, err = libp2p.New(
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", rc.Me.Host, rc.Me.Port)),
		libp2p.Transport(libp2pquic.NewTransport),
		libp2p.DefaultTransports,
	)
	if err != nil {
		panic(err)
	}
	// print the node's listening addresses
	fmt.Println("Listen addresses:", rc.Host.Addrs())
	handleErr(err)

	rc.Peers = append(rc.Peers[:myIdx], rc.Peers[myIdx+1:]...)
	go rc.daemon()
}

func Make() *RC {
	enc, err := reedsolomon.New(*dataShard, *parShard)
	handleErr(err)
	rc = &RC{
		Enc:          enc,
		Ctx:          context.Background(),
		Me:           Node{},
		Host:         nil,
		Peers:        make([]Node, 0),
		RFile:        []RFile{},
		TranCh:       make([]chan interface{}, 0),
		TranProtocol: "//transfer",
	}
	rc.ReadConfig()
	return rc
}
