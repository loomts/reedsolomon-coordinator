package main

import (
	"github/loomts/reedsolomon-coordinator/coordinator"
	"github/loomts/reedsolomon-coordinator/utils"
)

func main() {
	utils.LogInit()
	coordinator.Start()
}
