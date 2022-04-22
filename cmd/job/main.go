package main

import (
	"context"
	"encoding/json"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-co-op/gocron"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"math/big"
	"net/http"
	"nft-event/contracts"
	"nft-event/db"
	"nft-event/model"
	"nft-event/util"
	"os"
	"os/signal"
	"sync"
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

	log.Info("start nft event job")
	ethClient, err := ethclient.Dial(config.EthUri)
	if err != nil {
		log.Fatal(err)
	}

	mongoClient, ctx, cancel, err := db.Connect(config.MongoUri)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close(mongoClient, ctx, cancel)

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

	nftMap := make(map[common.Address]*contracts.Token, len(nfts))
	for _, nft := range nfts {
		instance, err := contracts.NewToken(common.HexToAddress(nft.Address), ethClient)
		if err != nil {
			log.Info("failed to create token instance")
			continue
		}
		nftMap[common.HexToAddress(nft.Address)] = instance
	}

	for _, nft := range nfts {
		c := gocron.NewScheduler(time.Local)
		_, _ = c.Every(10).Seconds().Do(func() { handleNftEvent(ethClient, &nft, mongoClient, config, nftMap) })
		c.StartAsync()
	}

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
}

func handleNftEvent(ethClient *ethclient.Client, nft *model.Nft, client *mongo.Client, config *util.Config, nftMap map[common.Address]*contracts.Token) {
	// current block
	header, err := ethClient.HeaderByNumber(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}
	currentBlock := header.Number.Int64()
	log.Infof("current block number: %d\n", currentBlock)
	log.Infof("nftId: %d", nft.NftId)

	result := model.Block{}
	collection := client.Database(config.MongoDb).Collection(config.MongoBlock)
	err = collection.FindOne(context.Background(), bson.D{{"nftId", nft.NftId}}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Infof("No blocks for nftId %d", nft.NftId)
			return
		}
		log.Error(err)
	}
	log.Infof("block %d - %d", result.Current, currentBlock)

	// Update max 200 blocks at one time
	diff := currentBlock - result.Current
	if diff > 200 {
		currentBlock = result.Current + 200
	}

	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(result.Current),
		ToBlock:   big.NewInt(currentBlock),
		Addresses: []common.Address{
			common.HexToAddress(nft.Address),
		},
	}

	logs, err := ethClient.FilterLogs(context.Background(), query)
	if err != nil {
		log.Error(err)
	}

	var wg sync.WaitGroup
	for _, vLog := range logs {
		wg.Add(1)
		go asyncStore(vLog, client, config, nft, nftMap, &wg)

	}
	wg.Wait()

	// block doc
	blockDoc := bson.D{
		{"current", currentBlock},
		{"updatedAt", time.Now()},
	}

	_, err = db.UpdateOne(client, context.Background(), config.MongoDb, config.MongoBlock, blockDoc, bson.M{"nftId": nft.NftId})
	if err != nil {
		log.Error(err)
	}
	log.Info("end nft event job")
}

func asyncStore(vLog types.Log, client *mongo.Client, config *util.Config, nft *model.Nft, nftMap map[common.Address]*contracts.Token, wg *sync.WaitGroup) {
	defer wg.Done()

	// Transfer(from, to, tokenId)
	nftTransferSig := []byte("Transfer(address,address,uint256)")
	nftTransferSigHash := crypto.Keccak256Hash(nftTransferSig)

	switch vLog.Topics[0].Hex() {
	case nftTransferSigHash.Hex():
		from := "0x" + vLog.Topics[1].Hex()[26:]
		to := "0x" + vLog.Topics[2].Hex()[26:]
		tokenId, err := util.ConvertHexToInt(vLog.Topics[3].Hex())
		if err != nil {
			log.Error(err)
		}

		// transfer doc
		transferDoc := bson.D{
			{"tx", vLog.TxHash.String()},
			{"from", from},
			{"to", to},
			{"tokenId", tokenId},
			{"createdAt", time.Now()},
		}
		log.Infof("%v", transferDoc)

		_, err = db.InsertOne(client, context.Background(), config.MongoDb, config.MongoEvent, transferDoc)
		if err != nil {
			log.Error(err)
		}

		if instance, ok := nftMap[common.HexToAddress(nft.Address)]; ok {
			owner, err := instance.OwnerOf(&bind.CallOpts{}, big.NewInt(tokenId))
			if err != nil {
				log.Error(err)
				break
			}

			tokenUri, err := instance.TokenURI(&bind.CallOpts{}, big.NewInt(tokenId))
			if err != nil {
				log.Error(err)
				break
			}

			data, err := util.GetRequest(tokenUri)
			if err != nil {
				log.Error(err)
				break
			}

			var nftItem model.NftItem
			if err = json.Unmarshal(data, &nftItem); err != nil {
				log.Error(err)
				break
			}

			imageData, err := util.GetRequest(nftItem.Image)
			if err != nil {
				log.Error(err)
				break
			}
			mimeType := http.DetectContentType(imageData)

			// nft doc
			nftDoc := bson.D{
				{"nftAddress", nft.Address},
				{"tokenId", tokenId},
				{"owner", owner.String()},
				{"tokenUri", tokenUri},
				{"name", nftItem.Name},
				{"description", nftItem.Description},
				{"image", nftItem.Image},
				{"mimeType", mimeType},
				{"createdAt", time.Now()},
				{"updatedAt", time.Now()},
			}

			_, err = db.UpsertOne(client, context.Background(), config.MongoDb, config.MongoNft, nftDoc, bson.M{"nftAddress": nft.Address, "tokenId": tokenId})
			if err != nil {
				log.Error(err)
			}
		}
	}
}
