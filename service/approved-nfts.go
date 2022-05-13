package service

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"nft-event/contracts"
	"nft-event/model"
	"nft-event/util"
)

func GetApprovedNfts(ethClient *ethclient.Client, mongoClient *mongo.Client, config *util.Config) ([]common.Address, map[common.Address]*contracts.Token, error) {
	var addresses []common.Address
	nftMap := make(map[common.Address]*contracts.Token, 0)

	collection := mongoClient.Database(config.MongoDb).Collection(config.MongoApprovedNft)
	cur, err := collection.Find(context.Background(), bson.D{})
	if err != nil {
		return addresses, nftMap, err
	}

	defer func(cur *mongo.Cursor, ctx context.Context) {
		err := cur.Close(ctx)
		if err != nil {
			return
		}
	}(cur, context.Background())

	var nfts []model.Nft
	for cur.Next(context.Background()) {
		result := model.Nft{}
		err := cur.Decode(&result)
		if err != nil {
			return addresses, nftMap, err
		}
		nfts = append(nfts, result)
	}
	if err := cur.Err(); err != nil {
		return addresses, nftMap, err
	}

	nftMap = make(map[common.Address]*contracts.Token, len(nfts))

	for _, nft := range nfts {
		instance, err := contracts.NewToken(common.HexToAddress(nft.Address), ethClient)
		if err != nil {
			continue
		}
		nftMap[common.HexToAddress(nft.Address)] = instance
		addresses = append(addresses, common.HexToAddress(nft.Address))
	}

	return addresses, nftMap, nil
}
