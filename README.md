# reedsolomon-coordinator

This project use [reedsolomon](https://github.com/klauspost/reedsolomon.git) to coordinate the store of cluster.

## Usage

### config.yml

sockets

## Design

add file -> store metadata -> split file -> send block to peers.
get file -> check metadata -> get block from peers -> combine block.
