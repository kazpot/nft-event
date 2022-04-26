BIN_OUT = $(shell pwd)/bin

receiver:
	go run cmd/receiver/main.go

job:
	go run cmd/job/main.go

env.local:
	cp .env.local .env

env.rinkeby:
	cp .env.rinkeby .env

build:
	env GOOS=linux GOARCH=amd64 go build -o $(BIN_OUT)/nft-event-job cmd/job/main.go

deploy:
	make build
	scp ./bin/nft-event-job x:/home/xs668689/app
	scp ./.env x:/home/xs668689/app
	make clean

clean:
	rm -rf $(BIN_OUT)

.PHONY: receiver job env.local env.rinkeby