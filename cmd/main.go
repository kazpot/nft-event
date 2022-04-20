package main

import (
	"context"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"math/big"
	"nft-event/contracts"
	"nft-event/db"
	"nft-event/util"
	"time"
)

type Nft struct {
	ID        primitive.ObjectID `bson:"_id"`
	Address   string             `bson:"address"`
	CreatedAt primitive.DateTime `bson:"createdAt"`
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	config, err := util.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	ethClient, err := ethclient.Dial("ws://localhost:8545")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("start nft event service")

	// current block
	header, err := ethClient.HeaderByNumber(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("current block number: %s\n", header.Number.String())

	mongoClient, ctx, cancel, err := db.Connect(config.MongoUri)
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close(mongoClient, ctx, cancel)

	log.Println("connected to db successfully")

	collection := mongoClient.Database(config.MongoDb).Collection(config.MongoApprovedNft)
	cur, err := collection.Find(context.Background(), bson.D{})
	defer func(cur *mongo.Cursor, ctx context.Context) {
		err := cur.Close(ctx)
		if err != nil {

		}
	}(cur, context.Background())

	var addresses []common.Address
	for cur.Next(context.Background()) {
		result := Nft{}
		err := cur.Decode(&result)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("approved nfts")
		log.Println("address: " + result.Address)
		addresses = append(addresses, common.HexToAddress(result.Address))
	}
	if err := cur.Err(); err != nil {
		log.Fatal("failed to read collection")
	}

	nftMap := make(map[common.Address]*contracts.Token, len(addresses))
	for _, address := range addresses {
		instance, err := contracts.NewToken(address, ethClient)
		if err != nil {
			log.Println("failed to create token instance")
			continue
		}
		nftMap[address] = instance
	}

	// subscribe event
	query := ethereum.FilterQuery{
		Addresses: addresses,
	}

	logs := make(chan types.Log)
	sub, err := ethClient.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		log.Fatal(err)
	}

	// Transfer(from, to, tokenId)
	nftTransferSig := []byte("Transfer(address,address,uint256)")
	nftTransferSigHash := crypto.Keccak256Hash(nftTransferSig)

	// Approval(owner, approved, tokenId)
	nftApproveSig := []byte("Approval(address,address,uint256)")
	nftApproveSigHash := crypto.Keccak256Hash(nftApproveSig)

	var doc interface{}
	for {
		select {
		case err := <-sub.Err():
			log.Println(err)
		case vLog := <-logs:

			log.Printf("block number: %d\n", vLog.BlockNumber)

			nftAddress := vLog.Address
			log.Printf("nft address: %s\n", nftAddress.String())

			switch vLog.Topics[0].Hex() {
			case nftTransferSigHash.Hex():
				log.Printf("transfer event\n")
				log.Printf("tx: %s\n", vLog.TxHash.String())

				from := "0x" + vLog.Topics[1].Hex()[26:]
				to := "0x" + vLog.Topics[2].Hex()[26:]

				tokenId, err := util.ConvertHexToInt(vLog.Topics[3].Hex())
				if err != nil {
					log.Println("failed to convert")
				}

				doc = bson.D{
					{"tx", vLog.TxHash.String()},
					{"from", from},
					{"to", to},
					{"tokenId", tokenId},
					{"createdAt", time.Now()},
				}

				_, err = db.InsertOne(mongoClient, context.Background(), config.MongoDb, config.MongoEvent, doc)
				if err != nil {
					log.Println(err)
				}

				// check owner of this token
				if instance, ok := nftMap[nftAddress]; ok {
					owner, err := instance.OwnerOf(&bind.CallOpts{}, big.NewInt(tokenId))
					if err != nil {
						log.Printf("failed to owner of token: %s, id: %d", addresses[0], 1)
						break
					}

					doc = bson.D{
						{"nftAddress", nftAddress.String()},
						{"tokenId", tokenId},
						{"owner", owner.String()},
						{"updatedAt", time.Now()},
					}

					_, err = db.UpsertOne(mongoClient, context.Background(), config.MongoDb, config.MongoNft, doc, bson.M{"nftAddress": nftAddress, "tokenId": tokenId})
					if err != nil {
						log.Println(err)
					}
				} else {
					log.Printf("address is not in nft map: %s\n", nftAddress.String())
				}
			case nftApproveSigHash.Hex():
				log.Printf("approval event\n")
				log.Printf("tx: %s\n", vLog.TxHash.String())

				ownerAddress := "0x" + vLog.Topics[1].Hex()[26:]
				log.Printf("owner address: %s\n", ownerAddress)

				approvedAddress := "0x" + vLog.Topics[2].Hex()[26:]
				log.Printf("approved address: %s\n", approvedAddress)

				value, err := util.ConvertHexToInt(vLog.Topics[3].Hex())
				if err != nil {
					log.Println("failed to convert")
				}
				log.Printf("tokenId: %d\n", value)
			default:
				log.Printf("other event\n")
				log.Printf("event Hash: %v\n", vLog.Topics[0].Hex())
			}
		}
	}
}
