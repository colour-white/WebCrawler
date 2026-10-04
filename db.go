package main

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func connectToMongoDB(uri string) (*mongo.Client, error) {

	client, err := mongo.Connect(
		options.Client().ApplyURI(
			uri,
		),
	)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func createIndex(client *mongo.Client, dbName string, ctx context.Context) (*mongo.Database, error) {

	db := client.Database(dbName)
	webpages := db.Collection("webpages")

	_, err := webpages.Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			// Keys: map[string]interface{}{
			// 	"title":   "text",
			// 	"content": "text",
			// },
			Keys: bson.D{
				{Key: "title", Value: "text"},
				{Key: "content", Value: "text"},
			},
		},
	)

	if err != nil {
		return nil, err
	}

	return db, nil

}

func insertParsedPage(db *mongo.Database, ctx context.Context, page *ParsedPage) error {

	webpages := db.Collection("webpages")

	_, err := webpages.InsertOne(ctx, bson.D{
		{Key: "title", Value: page.Title},
		{Key: "content", Value: page.Content},
		{Key: "url", Value: page.Url},
	})

	return err
}
