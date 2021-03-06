package model

import "go.mongodb.org/mongo-driver/bson/primitive"

type Nft struct {
	ID        primitive.ObjectID `bson:"_id"`
	NftId     int64              `bson:"nftId"`
	Address   string             `bson:"address"`
	CreatedAt primitive.DateTime `bson:"createdAt"`
}
