package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDB wraps a MongoDB client with optimized bulk operations
type MongoDB struct {
	client *mongo.Client
}

// NewMongoDB creates a new MongoDB client with connection pooling
// optimized for high-throughput workloads (200K+ inserts/sec target)
func NewMongoDB(ctx context.Context, uri string) (*MongoDB, error) {
	// Configure client with aggressive connection pooling
	clientOpts := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(100).        // 100 connections for parallel writes
		SetMinPoolSize(10).          // Keep 10 connections warm
		SetMaxConnIdleTime(5 * time.Minute).
		SetCompressors([]string{"snappy", "zstd"}). // Wire compression for network efficiency
		SetConnectTimeout(10 * time.Second).
		SetServerSelectionTimeout(5 * time.Second)

	// Connect to MongoDB
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	// Verify connectivity with ping
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("ping: %w", err)
	}

	return &MongoDB{client: client}, nil
}

// Close gracefully disconnects the MongoDB client
func (db *MongoDB) Close(ctx context.Context) error {
	return db.client.Disconnect(ctx)
}

// Ping checks if MongoDB is reachable
func (db *MongoDB) Ping(ctx context.Context) error {
	return db.client.Ping(ctx, nil)
}

// BulkInsert performs unordered bulk insert for maximum throughput
// Uses BulkWrite with unordered execution to parallelize writes across shards
func (db *MongoDB) BulkInsert(ctx context.Context, database, collection string, docs []map[string]any) error {
	if len(docs) == 0 {
		return nil
	}

	coll := db.client.Database(database).Collection(collection)

	const batchSize = 1000

	for i := 0; i < len(docs); i += batchSize {
		end := min(i+batchSize, len(docs))
		batch := docs[i:end]

		// Build write models for BulkWrite
		models := make([]mongo.WriteModel, len(batch))
		for j, doc := range batch {
			models[j] = mongo.NewInsertOneModel().SetDocument(doc)
		}

		// Execute bulk write with unordered mode (parallel execution)
		// This allows MongoDB to continue even if some writes fail
		opts := options.BulkWrite().SetOrdered(false)
		result, err := coll.BulkWrite(ctx, models, opts)
		if err != nil {
			// Check if it's a BulkWriteException (partial failure)
			if bulkErr, ok := err.(mongo.BulkWriteException); ok {
				// Some writes succeeded, log and continue
				_ = result // Use result to track successful inserts
				return fmt.Errorf("bulk write partial failure: %d inserted, %d errors: %w",
					result.InsertedCount, len(bulkErr.WriteErrors), err)
			}
			return fmt.Errorf("bulk write: %w", err)
		}
	}

	return nil
}

// InsertOne inserts a single document
func (db *MongoDB) InsertOne(ctx context.Context, database, collection string, doc map[string]any) error {
	coll := db.client.Database(database).Collection(collection)
	_, err := coll.InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("insert one: %w", err)
	}
	return nil
}

// InsertMany inserts multiple documents in a single operation
func (db *MongoDB) InsertMany(ctx context.Context, database, collection string, docs []map[string]any) error {
	if len(docs) == 0 {
		return nil
	}

	coll := db.client.Database(database).Collection(collection)

	// Convert to []interface{} as required by InsertMany
	interfaceDocs := make([]interface{}, len(docs))
	for i, doc := range docs {
		interfaceDocs[i] = doc
	}

	opts := options.InsertMany().SetOrdered(false) // Parallel execution
	_, err := coll.InsertMany(ctx, interfaceDocs, opts)
	if err != nil {
		return fmt.Errorf("insert many: %w", err)
	}

	return nil
}

// CreateCollection creates a new collection (optional, MongoDB auto-creates)
func (db *MongoDB) CreateCollection(ctx context.Context, database, collection string) error {
	err := db.client.Database(database).CreateCollection(ctx, collection)
	if err != nil {
		return fmt.Errorf("create collection: %w", err)
	}
	return nil
}

// ListCollections returns all collection names in a database
func (db *MongoDB) ListCollections(ctx context.Context, database string) ([]string, error) {
	names, err := db.client.Database(database).ListCollectionNames(ctx, map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	return names, nil
}

// DropCollection deletes a collection
func (db *MongoDB) DropCollection(ctx context.Context, database, collection string) error {
	err := db.client.Database(database).Collection(collection).Drop(ctx)
	if err != nil {
		return fmt.Errorf("drop collection: %w", err)
	}
	return nil
}

// GetCollection returns a collection handle for advanced operations
func (db *MongoDB) GetCollection(database, collection string) *mongo.Collection {
	return db.client.Database(database).Collection(collection)
}
