package model

import "go.mongodb.org/mongo-driver/bson/primitive"

type Block struct {
	ID        primitive.ObjectID `bson:"_id"`
	NftId     int64              `bson:"nftId"`
	Current   int64              `bson:"current"`
	UpdatedAt primitive.DateTime `bson:"updatedAt"`
	CreatedAt primitive.DateTime `bson:"createdAt"`
}
