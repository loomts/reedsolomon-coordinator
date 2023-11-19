package main

import (
	"net"
	"os"
	"path"
)

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

func pwd() string {
	base, _ := os.Getwd()
	return base
}

func pwdJoin(file string) string {
	return path.Join(pwd(), file)
}

func defaultStore() string {
	home, _ := os.UserHomeDir()
	return path.Join(home, ".reedsolomon-coordinator")
}
