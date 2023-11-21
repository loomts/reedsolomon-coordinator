package utils

import (
	"flag"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"os"
	"strings"
)

type Node struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	ID   peer.ID
}
type Config struct {
	Data  int    `mapstructure:"data"`
	Par   int    `mapstructure:"par"`
	Port  int    `mapstructure:"p"`
	Dest  string `mapstructure:"d"`
	Peers []Node `mapstructure:"nodes"` // peers
}

var data = flag.Int("data", 4, "Number of shards to split the data into, must be below 257.")
var par = flag.Int("par", 2, "Number of parity shards")
var port = flag.Int("p", 0, "Source port number")
var dest = flag.String("d", "", "Destination multiaddr string")

func ConfigInit() (cfg Config) {
	flag.Parse()
	viper.SetConfigFile("config.yml")
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	err := viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		log.Errorf("viper BindPFlags fail: %s", err)
	}
	err = viper.ReadInConfig()
	if err != nil {
		log.Errorf("viper init fail: %s", err)
	}
	if err = viper.Unmarshal(&cfg); err != nil {
		log.Errorf("viper unmarshal fail: %s", err)
	}
	if (*data + *par) > 256 {
		log.Error("Error: sum of data and parity shards cannot exceed 256")
		os.Exit(1)
	}

	for i, node := range cfg.Peers {
		// replace environment variables
		if strings.HasPrefix(node.Host, "${") && strings.HasSuffix(node.Host, "}") {
			cfg.Peers[i].Host = os.Getenv(strings.TrimSuffix(strings.TrimPrefix(node.Host, "${"), "}"))
		}
		//fmt.Printf("host:%v port:%v\n", rc.Peers[i].Host, rc.Peers[i].Port)
	}
	return cfg
}
