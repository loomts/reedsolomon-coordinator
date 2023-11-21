# reedsolomon-coordinator

This project use [reedsolomon](https://github.com/klauspost/reedsolomon.git) to coordinate the store of cluster.

## Usage

## Design

add file -> store metadata -> split file -> erasure encoding -> send shards to peers.
get file -> check metadata -> get shards from peers -> erasure decoding(if necessary) -> write bytes to file.

## TODO

- [x] optimize project structure
- [ ] use ipfs to store shards
- [ ] add reconstruct file option
- [ ] add mdns and dht
- [ ] multi stream(concurrence) to send one file
- [ ] reuse stream pool
- [ ] optimize Reed-Solomon Codes
