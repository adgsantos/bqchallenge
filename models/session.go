package models

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const kvColl = "keyValue"

type KV struct {
	Key   string `bson:"key"`
	Value string `bson:"value"`
}

type Event struct {
	Kv        KV     `bson:"kv"`
	Op        string `bson:"op"`
	Timestamp int    `bson:"timestamp"`
}

func assertNotErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

type Session struct {
	db  *mongo.Database
	ctx context.Context
}

var AlreadyExists = status.Errorf(codes.AlreadyExists, "Key already exists")
var NotFound = status.Errorf(codes.NotFound, "Key not found")

func newMongoClient(url string) (*mongo.Client, context.Context) {
	client, err := mongo.NewClient(options.Client().ApplyURI(url))
	assertNotErr(err)

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)

	err = client.Connect(ctx)
	assertNotErr(err)
	return client, context.Background()
}

func (s *Session) execTransaction(callback func(sessionContext mongo.SessionContext) (interface{}, error)) (interface{}, error) {
	session, err := s.db.Client().StartSession()
	ctx := s.ctx
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)

	return session.WithTransaction(ctx, callback)
}

func (s *Session) createIndex(colName string, index mongo.IndexModel) {
	collection := s.db.Collection(colName)

	opts := options.CreateIndexes().SetMaxTime(10 * time.Second)
	_, errIndex := collection.Indexes().CreateOne(s.ctx, index, opts)
	assertNotErr(errIndex)
}

func NewSession(url string, dbName string) *Session {
	client, ctx := newMongoClient(url)
	return &Session{db: client.Database(dbName), ctx: ctx}
}

func (s *Session) Close() {
	if err := s.db.Client().Disconnect(s.ctx); err != nil {
		panic(err)
	}
}

func (s *Session) ResetDB() {
	s.db.Drop(s.ctx)

	kvIndex := mongo.IndexModel{
		Keys: &bson.D{
			{Key: "Key", Value: "hashed"},
			{Key: "Timestamp", Value: -1}},
	}
	s.createIndex(kvColl, kvIndex)
}

func (s *Session) fetchLatestEvent(coll *mongo.Collection, key string) (*Event, error) {

	var ev *Event
	err := coll.FindOne(
		s.ctx,
		bson.D{{"Kv.Key", key}},
		options.FindOne().SetSort(bson.D{{"Timestamp", -1}}),
	).Decode(&ev)

	return ev, err
}

func (s *Session) insertEvent(coll *mongo.Collection, key string, value string, ev string) error {
	_, err := coll.InsertOne(s.ctx, bson.D{
		{"Kv", bson.D{{"Key", key}, {"Value", value}}},
		{"Timestamp", time.Now().Nanosecond()},
		{"Op", ev},
	})
	return err
}

func (s *Session) Create(key string, value string) error {
	coll := s.db.Collection(kvColl)

	cb := func(sCtx mongo.SessionContext) (interface{}, error) {

		ev, err := s.fetchLatestEvent(coll, key)

		// For simplicity, we only allow mongo.ErrNoDocuments to be propagated downstream
		if err != nil && err != mongo.ErrNoDocuments {
			panic(err)
		}

		if err == nil && ev.Op != "delete" {
			// It exists and it is still active
			return nil, AlreadyExists
		}

		err = s.insertEvent(coll, key, value, "create")
		if err != nil {
			panic(err)
		}
		return nil, nil
	}

	_, err := s.execTransaction(cb)
	return err
}

func (s *Session) Get(key string) (*KV, error) {
	coll := s.db.Collection(kvColl)

	ev, err := s.fetchLatestEvent(coll, key)

	if err == mongo.ErrNoDocuments || ev.Op == "delete" {
		return nil, NotFound
	}

	if err != nil {
		panic(err)
	}

	return &ev.Kv, nil
}

func (s *Session) Update(key string, value string) error {
	coll := s.db.Collection(kvColl)

	cb := func(sCtx mongo.SessionContext) (interface{}, error) {

		ev, err := s.fetchLatestEvent(coll, key)

		// For simplicity, we only allow mongo.ErrNoDocuments to be propagated downstream
		if err != nil && err != mongo.ErrNoDocuments {
			panic(err)
		}

		if err == mongo.ErrNoDocuments || ev.Op == "delete" {
			return nil, NotFound
		}

		err = s.insertEvent(coll, key, value, "update")
		if err != nil {
			panic(err)
		}
		return nil, nil
	}

	_, err := s.execTransaction(cb)
	return err
}

func (s *Session) Delete(key string) error {
	coll := s.db.Collection(kvColl)

	cb := func(sCtx mongo.SessionContext) (interface{}, error) {

		ev, err := s.fetchLatestEvent(coll, key)

		// For simplicity, we only allow mongo.ErrNoDocuments to be propagated downstream
		if err != nil && err != mongo.ErrNoDocuments {
			panic(err)
		}

		if err == mongo.ErrNoDocuments || ev.Op == "delete" {
			return nil, NotFound
		}

		err = s.insertEvent(coll, key, ev.Kv.Value, "delete")
		if err != nil {
			panic(err)
		}
		return nil, nil
	}

	_, err := s.execTransaction(cb)
	return err
}

func (s *Session) GetHistory(key string, limit int64) ([]Event, error) {
	coll := s.db.Collection(kvColl)

	evs := make([]Event, limit)
	cur, err := coll.Find(
		s.ctx,
		bson.D{{"Kv.Key", key}},
		options.Find().SetSort(bson.D{{"Timestamp", -1}}),
		options.Find().SetLimit(limit),
	)

	err = cur.All(s.ctx, &evs)
	if err != nil {
		panic(err)
	}

	return evs, nil
}
