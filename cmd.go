package main

import (
	"flag"
	"fmt"
	"os"
)

var rc *RC
var dataShard = flag.Int("data", 4, "Number of shards to split the data into, must be below 257.")
var parShard = flag.Int("par", 2, "Number of parity shards")
var outDir = flag.String("out", "", "Alternative output directory")

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  simple-encoder [-flags] filename.ext\n")
		fmt.Fprintf(os.Stderr, "Valid flags:\n")
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()
	args := flag.Args()
	if (*dataShard + *parShard) > 256 {
		fmt.Fprintln(os.Stderr, "Error: sum of data and parity shards cannot exceed 256")
		os.Exit(1)
	}
	fmt.Println(len(args))
	if len(args) == 0 {
		rc = Make()
	} else {
		if rc == nil {
			panic("Error: before use first init coordinator without any args")
		}
		if len(args) == 1 {
			rc.add(args[1])
		} else {
			rc.get(args[2])
		}
	}
}
