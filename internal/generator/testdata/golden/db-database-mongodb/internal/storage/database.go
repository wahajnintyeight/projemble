package storage

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func Open(ctx context.Context, databaseURL string) (*mongo.Client, error) {
	return mongo.Connect(options.Client().ApplyURI(databaseURL))
}
