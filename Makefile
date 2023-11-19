build:
	GOOS=windows GOARCH=amd64 go build -o rsc.exe reedsolomon-coordinator
	cp /home/loomt/gopath/src/reedsolomon-coordinator/rsc.exe /mnt/f/Desktop/rsc/rsc.exe
	cp /home/loomt/gopath/src/reedsolomon-coordinator/config.yml /mnt/f/Desktop/rsc/config.yml
	./rsc.exe
clean:
	rm ./rsc.exe