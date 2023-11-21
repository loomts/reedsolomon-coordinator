package utils

import (
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"time"
)

func LogInit() {
	zapCfg := zap.NewProductionEncoderConfig()
	zapCfg.EncodeTime = func(t time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		str := t.Format("15:04:05")
		encoder.AppendString(str)
	}
	logging.SetPrimaryCore(zapcore.NewCore(zapcore.NewConsoleEncoder(zapCfg), zapcore.Lock(os.Stdout), zap.NewAtomicLevel()))
}
