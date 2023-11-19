package main

import (
	"flag"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"time"
)

var log = logging.Logger("RS-Coordinator")

var data = flag.Int("data", 4, "Number of shards to split the data into, must be below 257.")
var par = flag.Int("par", 2, "Number of parity shards")
var port = flag.Int("p", 0, "Source port number")
var dest = flag.String("d", "", "Destination multiaddr string")

func logInit() {
	zapCfg := zap.NewProductionEncoderConfig()
	zapCfg.EncodeTime = func(t time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		//str := t.Format("2006-01-02 15:04:05")
		encoder.AppendString("")
	}
	logging.SetPrimaryCore(zapcore.NewCore(zapcore.NewConsoleEncoder(zapCfg), zapcore.Lock(os.Stdout), zap.NewAtomicLevel()))
}

func main() {
	logInit()
	flag.Parse()
	if (*data + *par) > 256 {
		log.Errorf("Error: sum of data and parity shards cannot exceed 256")
		os.Exit(1)
	}
	start()
}
