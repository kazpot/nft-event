package main

import (
	"context"
	"encoding/json"
	"errors"
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
	"io"
	"math/big"
	"net/http"
	"nft-event/contracts"
	"nft-event/db"
	"nft-event/model"
	"nft-event/util"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

// HexBytes ERC721 interface must be compliant with 0x80ac58cd
var HexBytes = [4]byte{0x80, 0xac, 0x58, 0xcd}
var BlockRange int64 = 1000

func main() {
	config, err := util.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	log.SetLevel(log.DebugLevel)
	log.SetReportCaller(true)

	if config.LogOutput {
		var file *os.File
		logName := config.LogName
		if _, err := os.Stat(logName); errors.Is(err, os.ErrNotExist) {
			file, err = os.Create(logName)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			file, err = os.OpenFile(logName, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
			if err != nil {
				log.Fatal(err)
			}
		}

		log.SetOutput(io.MultiWriter(file, os.Stdout))

		defer func(file *os.File) {
			err := file.Close()
			if err != nil {

			}
		}(file)
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

	c := gocron.NewScheduler(time.Local)
	_, _ = c.Every(1).Seconds().Do(func() { handleNftEvent(ethClient, mongoClient, config) })
	c.SingletonMode().StartBlocking()

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
}

func handleNftEvent(ethClient *ethclient.Client, client *mongo.Client, config *util.Config) {
	start := time.Now()

	// current block
	header, err := ethClient.HeaderByNumber(context.Background(), nil)
	if err != nil {
		log.Error(err)
		return
	}
	currentBlock := header.Number.Int64()
	log.Infof("current block number: %d", currentBlock)

	result := model.Block{}
	collection := client.Database(config.MongoDb).Collection(config.MongoBlock)
	err = collection.FindOne(context.Background(), bson.D{{"nftId", 1}}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Infof("No blocks for nftId %d", 1)
			return
		}
		log.Error(err)
	}

	// Update max 200 blocks at one time
	diff := currentBlock - result.Current
	if diff > BlockRange {
		currentBlock = result.Current + BlockRange
	}

	if diff == 0 {
		return
	} else {
		log.Infof("block %d - %d", result.Current, currentBlock)
	}

	query := ethereum.FilterQuery{
		Addresses: []common.Address{common.HexToAddress(config.NftAddress)},
		FromBlock: big.NewInt(result.Current),
		ToBlock:   big.NewInt(currentBlock),
	}

	logs, err := ethClient.FilterLogs(context.Background(), query)
	if err != nil {
		log.Error(err)
	}

	log.Infof("number of event log %d", len(logs))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for _, vLog := range logs {
		wg.Add(1)
		go asyncStore(ethClient, vLog, client, config, &wg, ctx)
	}
	wg.Wait()

	// block doc
	blockDoc := bson.D{
		{"current", currentBlock},
		{"updatedAt", time.Now()},
	}

	_, err = db.UpdateOne(client, context.Background(), config.MongoDb, config.MongoBlock, blockDoc, bson.M{"nftId": 1})
	if err != nil {
		log.Error(err)
	}

	duration := time.Since(start)
	log.Infof("end nft event job, duration: %.2f", duration.Seconds())
}

func asyncStore(ethClient *ethclient.Client, vLog types.Log, client *mongo.Client, config *util.Config, wg *sync.WaitGroup, ctx context.Context) {
	defer wg.Done()

	ch := make(chan string)
	go func() {
		defer close(ch)
		vlogStart := time.Now()

		// Transfer(from, to, tokenId)
		nftTransferSig := []byte("Transfer(address,address,uint256)")
		nftTransferSigHash := crypto.Keccak256Hash(nftTransferSig)

		// skip erc20 transfer event which has 3 topics
		if len(vLog.Topics) != 4 {
			return
		}

		switch vLog.Topics[0].Hex() {
		case nftTransferSigHash.Hex():
			nftAddress := vLog.Address.String()
			instance, err := contracts.NewToken(common.HexToAddress(nftAddress), ethClient)

			from := "0x" + vLog.Topics[1].Hex()[26:]
			to := "0x" + vLog.Topics[2].Hex()[26:]
			tokenId, err := util.ConvertHexToBigInt(vLog.Topics[3].Hex())
			if err != nil {
				log.Error(err)
			}

			isErc721, err := instance.SupportsInterface(&bind.CallOpts{}, HexBytes)
			if err != nil || !isErc721 {
				log.Info("no erc721 compliant...")
				return
			}

			tokenUriStart := time.Now()
			tokenUri, err := instance.TokenURI(&bind.CallOpts{}, tokenId)
			if err != nil {
				log.Error(err)
				break
			}
			tokenUriStartDuration := time.Since(tokenUriStart)
			log.Infof("token uri end, duration: %.2f", tokenUriStartDuration.Seconds())

			ownerOfStart := time.Now()
			owner, err := instance.OwnerOf(&bind.CallOpts{}, tokenId)
			if err != nil {
				log.Error(err)
				break
			}
			ownerOfDuration := time.Since(ownerOfStart)
			log.Infof("owner of end, duration: %.2f", ownerOfDuration.Seconds())

			// TODO: skip except for http
			if !strings.HasPrefix(tokenUri, "http") {
				break
			}

			httpStart := time.Now()
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

			// TODO: skip except for http
			if !strings.HasPrefix(nftItem.Image, "http") {
				break
			}

			imageData, err := util.GetRequest(nftItem.Image)
			if err != nil {
				log.Error(err)
				break
			}
			mimeType := http.DetectContentType(imageData)

			httpDuration := time.Since(httpStart)
			log.Infof("http end, duration: %.2f", httpDuration.Seconds())

			// event doc
			transferDoc := bson.D{
				{"tx", vLog.TxHash.String()},
				{"nftAddress", nftAddress},
				{"from", from},
				{"to", to},
				{"tokenId", tokenId.String()},
				{"createdAt", time.Now()},
			}
			log.Infof("%v", transferDoc)

			_, err = db.InsertOne(client, context.Background(), config.MongoDb, config.MongoEvent, transferDoc)
			if err != nil {
				log.Error(err)
			}

			// nft doc
			nftDoc := bson.D{
				{"nftAddress", nftAddress},
				{"tokenId", tokenId.String()},
				{"owner", owner.String()},
				{"tokenUri", tokenUri},
				{"name", nftItem.Name},
				{"description", nftItem.Description},
				{"image", nftItem.Image},
				{"mimeType", mimeType},
				{"createdAt", time.Now()},
				{"updatedAt", time.Now()},
			}

			log.Infof("nft doc: %v", nftDoc)

			_, err = db.UpsertOne(client, context.Background(), config.MongoDb, config.MongoNft, nftDoc, bson.M{"nftAddress": nftAddress, "tokenId": tokenId})
			if err != nil {
				log.Error(err)
			}
		}

		select {
		case ch <- "done":
			vlogDuration := time.Since(vlogStart)
			log.Infof("vlog topics end, duration: %.5f", vlogDuration.Seconds())
		default:
			return
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("asyncStore timeout")
		return
	case <-ch:
		log.Info("asyncStore finished")
		return
	}
}
