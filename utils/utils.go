package utils

import (
	logging "github.com/ipfs/go-log/v2"
	"net"
	"os"
	"path"
)

var log = logging.Logger("utils")

func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Errorf("get net interface address failed, %s", err)
		return ""
	}
	for _, addr := range addrs {
		if ip, ok := addr.(*net.IPNet); ok && !ip.IP.IsLoopback() {
			if ip.IP.To4() != nil {
				return ip.IP.String()
			}
		}
	}
	return ""
}

func Pwd() string {
	base, _ := os.Getwd()
	return base
}

func PwdJoin(file string) string {
	return path.Join(Pwd(), file)
}

func RCStore() string {
	home, _ := os.UserHomeDir()
	return path.Join(home, ".reedsolomon-coordinator")
}

func Mkdir(d string) {
	if _, err := os.Stat(d); err != nil {
		err = os.Mkdir(d, 0777)
		log.Error(err)
	}
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
