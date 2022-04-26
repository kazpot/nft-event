BIN_OUT = $(shell pwd)/bin

receiver:
	make local
	go run cmd/receiver/main.go

job:
	make local
	go run cmd/job/main.go

build:
	env GOOS=linux GOARCH=amd64 go build -o $(BIN_OUT)/nft-event-job cmd/job/main.go

deploy:
	make rinkeby
	make build
	scp $(BIN_OUT)/nft-event-job x:/home/xs668689/app
	scp ./.env x:/home/xs668689/app
	make clean

clean:
	rm -rf $(BIN_OUT)

local:
	cp .env.local .env

rinkeby:
	cp .env.rinkeby .env

.PHONY: receiver job local rinkeby build deploy clean
