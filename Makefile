BIN_OUT = $(shell pwd)/bin

local-env:
	cp .env.local .env

rinkeby-env:
	cp .env.rinkeby .env

build:
	env GOOS=linux GOARCH=amd64 go build -o $(BIN_OUT)/nft-event-job cmd/job/main.go

clean:
	rm -rf $(BIN_OUT)

rinkeby:
	make rinkeby-env
	make build
	scp $(BIN_OUT)/nft-event-job x:/home/xs668689/app
	scp ./.env x:/home/xs668689/app
	make clean

local:
	make local-env
	go run cmd/job/main.go

.PHONY: build clean local rinkeby
