package coordinator

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	logging "github.com/ipfs/go-log/v2"
	"github.com/klauspost/reedsolomon"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"github/loomts/reedsolomon-coordinator/rs"
	"github/loomts/reedsolomon-coordinator/utils"
	"io"
	"os"
	"path"
	"strconv"
	"sync"
	"time"
)

var log = logging.Logger("coordinator")

type RFile struct {
	MD5    string
	Name   string
	Size   int64
	RSize  int
	Parity int
	S2P    map[int]int // shard to peer
}

type RC struct {
	Enc          reedsolomon.Encoder
	Ctx          context.Context
	Cfg          utils.Config
	Host         host.Host
	Peers        []utils.Node `mapstructure:"nodes"` // peers
	RFile        []RFile
	TranCh       []chan interface{} // Transfer Channel -> transfer status
	TranProtocol protocol.ID
}

func Start() {
	cfg := utils.ConfigInit()
	enc, err := reedsolomon.New(cfg.Data, cfg.Par)
	if err != nil {
		log.Errorf("reedsolomon init fail: %s", err)
	}
	rc := &RC{
		Enc:          enc,
		Ctx:          context.Background(),
		Cfg:          cfg,
		Host:         nil,
		Peers:        make([]utils.Node, 1), // self is first peer
		RFile:        make([]RFile, 0),
		TranCh:       make([]chan interface{}, 0),
		TranProtocol: "/transfer",
	}
	rc.Init()
}

func (rc *RC) Init() {
	utils.Mkdir(utils.RCStore())
	rc.Peers[0].Host, rc.Peers[0].Port = utils.GetLocalIP(), rc.Cfg.Port
	var err error
	rc.Host, err = libp2p.New(
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", rc.Peers[0].Host, rc.Peers[0].Port)),
		libp2p.DefaultTransports,
	)
	fmt.Println(rc.Host.Network().ListenAddresses())
	if err != nil {
		log.Errorf("libp2p init fail %s", err)
	}
	if rc.Cfg.Dest == "" {
		log.Infof("RUN \n\n./reedsolomon-coordinator -d %s/p2p/%s\n\n on another console.\n", rc.Host.Addrs()[0], rc.Host.ID())
		rc.Host.SetStreamHandler(rc.TranProtocol, rc.streamHandler)
	} else {
		// connect to the given IPFS ID
		mAddr, err := multiaddr.NewMultiaddr(rc.Cfg.Dest)
		if err != nil {
			log.Errorf("NewMultiaddr fail: %s", err)
		}
		info, err := peer.AddrInfoFromP2pAddr(mAddr)
		if err != nil {
			log.Errorf("cannot convery mAddr to info: %s", err)
		}
		rc.Host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
		s, err := rc.Host.NewStream(rc.Ctx, info.ID, rc.TranProtocol)
		if err != nil {
			log.Errorf("NewStream fail: %s", err)
		}
		s.SetDeadline(time.Now().Add(time.Hour))
		go ReceFiles(s)
		go rc.SendFiles(s)
	}
	select {}
}

func (rc *RC) streamHandler(s network.Stream) {
	log.Infof("%s Start to handle Stream!", rc.Host.ID())
	go ReceFiles(s)
	go rc.SendFiles(s)
}

func ReceFiles(s network.Stream) {
	tr := tar.NewReader(s)
	for {
		hdr, err := tr.Next()
		log.Infof("receive %s", hdr.Name)
		if err == io.EOF {
			break
		} else if err != nil {
			log.Errorf("tar read fail: %s", err)
		}
		absFName := path.Join(utils.RCStore(), hdr.Name)
		file, err := os.Create(absFName)
		if err != nil {
			log.Errorf("create file fail: %s", err)
		}
		written, err := io.Copy(file, tr)
		fileInfo, err := file.Stat()
		fmt.Println(fileInfo.Size())
		log.Infof("After read file:%s for %d byte", hdr.Name, written)
		if err != nil {
			log.Error("transfer fail：%s", err)
			return
		}
		log.Infof(hdr.Name + " received success")
		err = file.Close()
		if err != nil {
			log.Infof(hdr.Name + "close fail")
			return
		}
	}
}

func (rc *RC) SendFiles(s network.Stream) {
	rc.Peers = append(rc.Peers, utils.Node{
		Host: "",
		Port: 0,
		ID:   rc.Host.Peerstore().Peers()[0],
	})
	stdReader := bufio.NewReader(os.Stdin)
	tw := tar.NewWriter(s)
	defer tw.Close()
	for {
		fmt.Printf("We're in %s, input file to erasure encode or decode...\n> ", utils.Pwd())
		fName, _ := stdReader.ReadString('\n')
		fName = fName[:len(fName)-1]
		absFName := fName
		if !path.IsAbs(fName) {
			absFName = utils.PwdJoin(fName)
		}
		log.Infof("erasuring file %s", absFName)
		exist, _ := utils.PathExists(absFName)
		var shards [][]byte
		if exist {
			shards = rs.ErasureCoding(rc.Enc, absFName)
		}
		//else {
		//	file = rs.ReConstruct(rc.Enc,shards,absFName)
		//}

		fInfo, err := os.Stat(absFName)
		if err != nil {
			log.Errorf("read file stat fail: %s", err)
		}
		rf := RFile{
			Name:   fInfo.Name(),
			Size:   fInfo.Size(),
			RSize:  rc.Cfg.Data + rc.Cfg.Par,
			Parity: rc.Cfg.Par,
			S2P:    make(map[int]int),
		}
		wg := sync.WaitGroup{}
		wg.Add(len(shards))
		for i := 0; i < len(shards); i++ {
			rf.S2P[i] = i % len(rc.Peers)
			sName := fName + "." + strconv.Itoa(i)
			rc.sendShard(&shards[i], i%len(rc.Peers), sName, tw, &wg)
		}
		wg.Wait()
		rc.RFile = append(rc.RFile, rf)
	}
}

func (rc *RC) sendShard(shard *[]byte, id int, sName string, tw *tar.Writer, wg *sync.WaitGroup) {
	defer wg.Done()
	if id == 0 {
		// self store
		sName = path.Join(utils.RCStore(), sName)
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
		err := tw.WriteHeader(&tar.Header{
			Name: sName,
			Size: int64(len(*shard)),
		})
		if err != nil {
			log.Errorf("%s Write tar hander fail,%s", sName, err)
		}
		if err != nil {
			log.Errorf("file readerr %v", err)
		}
		log.Infof("size of shard %s is %d", sName, len(*shard))
		written, err := io.Copy(tw, bytes.NewReader(*shard)) // TODO compare to io.CopyBuffer
		if err != nil {
			log.Errorf("%s io copy fail: %s", sName, err)
		}
		err = tw.Flush()
		if err != nil {
			log.Errorf("tar flush false %s", err)
		}
		log.Infof("writeen %d of %s", written, sName)
	}
}
