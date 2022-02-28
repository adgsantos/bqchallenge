package services

import (
	"bequest/models"
	"bequest/proto"
	"context"
	"fmt"
)

type KeyValueServer struct {
	proto.UnimplementedKeyValueStoreServer
	Session *models.Session
}

func (s *KeyValueServer) Get(ctx context.Context, req *proto.GetRequest) (*proto.GetResponse, error) {
	fmt.Println(req.GetKey())
	res, err := s.Session.Get(req.GetKey())
	if err != nil {
		return nil, err
	}

	return &proto.GetResponse{Key: res.Key, Value: res.Value}, nil
}

func (s *KeyValueServer) Create(ctx context.Context, req *proto.CreateRequest) (*proto.CreateResponse, error) {
	if err := s.Session.Create(req.GetKey(), req.GetValue()); err != nil {
		return nil, err
	}
	return &proto.CreateResponse{}, nil
}

func (s *KeyValueServer) Delete(ctx context.Context, req *proto.DeleteRequest) (*proto.DeleteResponse, error) {
	if err := s.Session.Delete(req.GetKey()); err != nil {
		return nil, err
	}
	return &proto.DeleteResponse{}, nil
}

func (s *KeyValueServer) Update(ctx context.Context, req *proto.UpdateRequest) (*proto.UpdateResponse, error) {
	if err := s.Session.Update(req.GetKey(), req.GetValue()); err != nil {
		return nil, err
	}
	return &proto.UpdateResponse{}, nil
}

func (s *KeyValueServer) GetHistory(ctx context.Context, req *proto.GetHistoryRequest) (*proto.GetHistoryResponse, error) {
	res, err := s.Session.GetHistory(req.GetKey(), 20)
	if err != nil {
		return nil, err
	}

	var events []*proto.GetHistoryResponse_Event
	for _, ev := range res {
		item := &proto.GetHistoryResponse_Event_Item{Key: ev.Kv.Key, Value: ev.Kv.Value}
		events = append(events, &proto.GetHistoryResponse_Event{Op: ev.Op, Item: item})
	}

	return &proto.GetHistoryResponse{Events: events}, nil
}
