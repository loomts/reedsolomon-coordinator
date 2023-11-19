package main

import (
	"bufio"
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
	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/viper"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

type Node struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	ID   peer.ID
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
	Host         host.Host
	Peers        []Node `mapstructure:"nodes"` // peers
	RFile        []RFile
	TranCh       []chan interface{} // Transfer Channel -> transfer status
	TranProtocol protocol.ID
}

func start() {
	enc, err := reedsolomon.New(*data, *par)
	if err != nil {
		log.Errorf("reedsolomon init fail: %s", err)
	}
	rc := &RC{
		Enc:          enc,
		Ctx:          context.Background(),
		Host:         nil,
		Peers:        make([]Node, 1), // self is first peer
		RFile:        make([]RFile, 0),
		TranCh:       make([]chan interface{}, 0),
		TranProtocol: "//transfer",
	}
	rc.Init()
}

func (rc *RC) Init() {
	mkdir()
	viper.SetConfigFile("config.yml")
	err := viper.ReadInConfig()
	if err != nil {
		log.Errorf("viper init fail: %s", err)
	}
	err = viper.UnmarshalKey("nodes", &rc.Peers)
	if err != nil {
		log.Errorf("viper unmarshal fail: %s", err)
	}
	// set peers message by config.yml
	for i, node := range rc.Peers {
		// replace environment variables
		if strings.HasPrefix(node.Host, "${") && strings.HasSuffix(node.Host, "}") {
			rc.Peers[i].Host = os.Getenv(strings.TrimSuffix(strings.TrimPrefix(node.Host, "${"), "}"))
		}
		//fmt.Printf("host:%v port:%v\n", rc.Peers[i].Host, rc.Peers[i].Port)
	}
	rc.Peers[0].Host, rc.Peers[0].Port = GetLocalIP(), *port
	rc.Host, err = libp2p.New(
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", rc.Peers[0].Host, rc.Peers[0].Port)),
		libp2p.DefaultTransports,
	)

	if *dest == "" {
		// set transfer channel
		var freePort string
		for _, la := range rc.Host.Network().ListenAddresses() {
			if p, err := la.ValueForProtocol(multiaddr.P_TCP); err == nil {
				freePort = p
				break
			}
		}
		if freePort == "" {
			log.Errorf("cannot find enable port.")
			return
		}
		log.Infof("RUN \n\n./reedsolomon-coordinator -d %s/p2p/%s\n\n on another console.\n", rc.Host.Addrs()[0], rc.Host.ID())
		rc.Host.SetStreamHandler(rc.TranProtocol, rc.streamHandler)
	} else {
		// connect to the given IPFS ID
		maddr, err := multiaddr.NewMultiaddr(*dest)
		if err != nil {
			log.Errorf("NewMultiaddr fail: %s", err)
		}
		info, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			log.Errorf("cannot convery maddr to info: %s", err)
		}
		rc.Host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
		s, err := rc.Host.NewStream(rc.Ctx, info.ID, rc.TranProtocol)
		if err != nil {
			log.Errorf("NewStream fail: %s", err)
		}
		rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
		go rc.readShard(rw)
		go rc.writeShard(rw)
	}
	select {}
}

func mkdir() {
	p := defaultStore()
	if _, err := os.Stat(p); err != nil {
		err = os.Mkdir(p, 0777)
		log.Error(err)
	}
}

func (rc *RC) streamHandler(s network.Stream) {
	log.Infof("%s start to handle Stream!", rc.Host.ID())
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
	go rc.readShard(rw)
	go rc.writeShard(rw)
}

func (rc *RC) readShard(rw *bufio.ReadWriter) {
	for {
		sName, _ := rw.ReadString('\n')
		sName = sName[:len(sName)-1]
		file, err := os.Create(path.Join(defaultStore(), sName))
		if err != nil {
			log.Error("cannot create file：%s", err)
			return
		}
		fmt.Println()
		log.Infof("Reading file:%s", sName)
		written, err := io.CopyBuffer(file, rw, make([]byte, 1024))
		fileInfo, err := file.Stat()
		fmt.Println(fileInfo.Size())
		log.Infof("After read file:%s for %d byte", sName, written)
		if err != nil {
			log.Error("transfer fail：%s", err)
			return
		}
		log.Infof(sName + "transfer success")
		err = file.Close()
		if err != nil {
			log.Infof(sName + "close fail")
			return
		}
	}
}

func (rc *RC) writeShard(rw *bufio.ReadWriter) {
	stdReader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("We're in %s, input file to erasure...\n> ", pwd())
		fName, _ := stdReader.ReadString('\n')
		fName = fName[:len(fName)-1]
		longFName := pwdJoin(fName)
		fmt.Println(longFName)
		bfile, err := os.ReadFile(longFName)
		if err != nil {
			log.Errorf("read file fail: %s", err)
		}
		shards, err := rc.Enc.Split(bfile)
		if err != nil {
			log.Errorf("erasure coding fail: %s", err)
		}
		err = rc.Enc.Encode(shards)
		if err != nil {
			log.Errorf("erasure coding fail: %s", err)
		}
		fDetail, err := os.Stat(longFName)
		if err != nil {
			log.Errorf("read file stat fail: %s", err)
		}
		rf := RFile{
			MD5:    hex.EncodeToString(md5.New().Sum(bfile)),
			name:   fName,
			Size:   fDetail.Size(),
			RSize:  *data + *par,
			Parity: *par,
			S2P:    make(map[int]int),
		}
		rc.Peers = append(rc.Peers, Node{
			Host: "",
			Port: 0,
			ID:   rc.Host.Peerstore().Peers()[0],
		})
		wg := sync.WaitGroup{}
		wg.Add(len(shards))
		for i := 0; i < len(shards); i++ {
			log.Infof("Shard%d Writing to Peer%d", i, i%len(rc.Peers))
			rf.S2P[i] = i % len(rc.Peers)
			sName := fName + "." + strconv.Itoa(i)
			rc.storeShard(&shards[i], i%len(rc.Peers), sName, rw, &wg)
		}
		wg.Wait()
		rc.RFile = append(rc.RFile, rf)
	}
}

func (rc *RC) storeShard(shard *[]byte, id int, sName string, rw *bufio.ReadWriter, wg *sync.WaitGroup) {
	defer wg.Done()
	if id == 0 {
		// self store
		sName = path.Join(defaultStore(), sName)
		file, err := os.OpenFile(sName, os.O_CREATE|os.O_RDWR, 0666)
		if err != nil {
			log.Errorf("Open file fail:%s", err)
		}
		_, err = file.Write(*shard)
		if err != nil {
			log.Errorf("Write file fail:%s", err)
		}
		err = file.Close()
		if err != nil {
			log.Errorf("Close file fail:%s", err)
		}
	} else {
		// peer store
		_, err := rw.WriteString(sName + "\n") // shard name
		if err != nil {
			log.Errorf("%s Write String fail,%s", sName, err)
		}
		err = rw.Flush()
		if err != nil {
			log.Errorf("%s Write String flush fail,%s", sName, err)
		}
		log.Infof("size of shard%s:%d", sName, len(*shard))
		written, err := io.CopyN(rw, bytes.NewReader(*shard), int64(len(*shard)))

		err = rw.Flush()
		if err != nil {
			log.Errorf("%s Write String flush fail,%s", sName, err)
		}
		if err != nil {
			log.Errorf("%s io copy fail: %s", sName, err)
		}
		log.Infof("writeen %d of %s", written, sName)
	}
}
