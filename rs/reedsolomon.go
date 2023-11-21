package rs

import (
	logging "github.com/ipfs/go-log/v2"
	"github.com/klauspost/reedsolomon"
	"os"
)

var log = logging.Logger("erasure-coding")

func ErasureCoding(enc reedsolomon.Encoder, fName string) [][]byte {
	log.Infof("erasuring file %s", fName)
	bfile, err := os.ReadFile(fName)
	if err != nil {
		log.Errorf("read file fail: %s", err)
	}
	shards, err := enc.Split(bfile)
	if err != nil {
		log.Errorf("erasure coding fail: %s", err)
	}
	err = enc.Encode(shards)
	if err != nil {
		log.Errorf("erasure coding fail: %s", err)
	}
	return shards
}

func ReConstruct(enc reedsolomon.Encoder, shards [][]byte, fName string) {
	ok, err := enc.Verify(shards)
	if ok {
		log.Infof("No need to reconstruction")
	} else {
		log.Infof("Verification failed %s. Reconstructing data", err)
		err = enc.Reconstruct(shards)
		if err != nil {
			log.Infof("Reconstruct failed %s", err)
		}
		ok, err = enc.Verify(shards)
		if !ok {
			log.Infof("Verification failed after reconstruction %s. Data likely corrupted.", err)
		}
	}
}
