package main

import (
	"context"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"math/big"
	"nft-event/contracts"
	"nft-event/db"
	"nft-event/model"
	"nft-event/util"
	"time"
)

func main() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	log.SetLevel(log.DebugLevel)
	log.SetReportCaller(true)

	config, err := util.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	ethClient, err := ethclient.Dial("ws://localhost:8545")
	if err != nil {
		log.Fatal(err)
	}

	log.Info("start nft event receiver")

	// current block
	header, err := ethClient.HeaderByNumber(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("current block number: %s\n", header.Number.String())

	mongoClient, ctx, cancel, err := db.Connect(config.MongoUri)
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close(mongoClient, ctx, cancel)

	log.Info("connected to db successfully")

	collection := mongoClient.Database(config.MongoDb).Collection(config.MongoApprovedNft)
	cur, err := collection.Find(context.Background(), bson.D{})
	defer func(cur *mongo.Cursor, ctx context.Context) {
		err := cur.Close(ctx)
		if err != nil {
			log.Error(err)
		}
	}(cur, context.Background())

	var nfts []model.Nft
	for cur.Next(context.Background()) {
		result := model.Nft{}
		err := cur.Decode(&result)
		if err != nil {
			log.Fatal(err)
		}
		log.Info("approved nfts")
		log.Infof("nftId: %d, address: %s", result.NftId, result.Address)
		nfts = append(nfts, result)
	}
	if err := cur.Err(); err != nil {
		log.Fatal("failed to read collection")
	}

	var addresses []common.Address
	nftMap := make(map[common.Address]*contracts.Token, len(nfts))
	for _, nft := range nfts {
		instance, err := contracts.NewToken(common.HexToAddress(nft.Address), ethClient)
		if err != nil {
			log.Info("failed to create token instance")
			continue
		}
		nftMap[common.HexToAddress(nft.Address)] = instance
		addresses = append(addresses, common.HexToAddress(nft.Address))
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
			log.Error(err)
		case vLog := <-logs:

			log.Infof("block number: %d\n", vLog.BlockNumber)

			nftAddress := vLog.Address
			log.Infof("nft address: %s\n", nftAddress.String())

			switch vLog.Topics[0].Hex() {
			case nftTransferSigHash.Hex():
				log.Infof("transfer event\n")
				log.Infof("tx: %s\n", vLog.TxHash.String())

				from := "0x" + vLog.Topics[1].Hex()[26:]
				to := "0x" + vLog.Topics[2].Hex()[26:]

				tokenId, err := util.ConvertHexToInt(vLog.Topics[3].Hex())
				if err != nil {
					log.Info("failed to convert")
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
					log.Error(err)
				}

				// check owner of this token
				if instance, ok := nftMap[nftAddress]; ok {
					owner, err := instance.OwnerOf(&bind.CallOpts{}, big.NewInt(tokenId))
					if err != nil {
						log.Infof("failed to owner of token: %s, id: %d", addresses[0], 1)
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
						log.Info(err)
					}
				} else {
					log.Infof("address is not in nft map: %s\n", nftAddress.String())
				}
			case nftApproveSigHash.Hex():
				log.Infof("approval event\n")
				log.Infof("tx: %s\n", vLog.TxHash.String())

				ownerAddress := "0x" + vLog.Topics[1].Hex()[26:]
				log.Infof("owner address: %s\n", ownerAddress)

				approvedAddress := "0x" + vLog.Topics[2].Hex()[26:]
				log.Infof("approved address: %s\n", approvedAddress)

				value, err := util.ConvertHexToInt(vLog.Topics[3].Hex())
				if err != nil {
					log.Info("failed to convert")
				}
				log.Infof("tokenId: %d\n", value)
			default:
				log.Infof("other event\n")
				log.Infof("event Hash: %v\n", vLog.Topics[0].Hex())
			}
		}
	}
}
