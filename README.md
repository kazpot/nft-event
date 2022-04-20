# NFT Event
Record current NFT owners and all transfer events for approved NFT addresses

# Install
Download go-ethereum, build and install the devtools (which includes abigen)
```
$ git clone https://github.com/ethereum/go-ethereum.git
$ cd go-ethereum
$ make devtools
```

Create Go bindings from an ABI file
```
$ abigen --abi=erc20_sol_ERC20.abi --pkg=token --out=erc20.go
```

Copy env.example
```
$ cp .env.example .env
```

Install go packages
```
$ go mod download
```