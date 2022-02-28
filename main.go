package main

import (
	"bequest/gateway"
	"bequest/insecure"
	"bequest/models"
	api "bequest/proto"
	"bequest/services"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"log"
	"net"
)

func main() {
	addr := fmt.Sprintf(":%d", 8000)
	conn, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Cannot listen to address %s", addr)
	}
	s := grpc.NewServer(
		grpc.Creds(credentials.NewServerTLSFromCert(&insecure.Cert)),
	)

	session := models.NewSession("mongodb://localhost:27021/?directConnection=true&serverSelectionTimeoutMS=2000", "test")
	session.ResetDB()
	defer session.Close()

	api.RegisterKeyValueStoreServer(s, &services.KeyValueServer{Session: session})

	fmt.Printf("Serving gRPC on %s\n", addr)
	go func() {
		if err := s.Serve(conn); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	if err := gateway.Run("dns:///" + addr); err != nil {
		log.Fatalf("Gateway: %v", err)
	}
}
