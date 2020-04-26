package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	"google.golang.org/grpc"
)

// MyServer -
type MyServer struct {
	mu sync.RWMutex
}

// NewMyServer -
func NewMyServer() *MyServer {
	return &MyServer{
		mu: sync.RWMutex{},
	}
}

// Logging -
func (srv *MyServer) Logging(n *Nothing, als Admin_LoggingServer) error {
	return nil
}

// Statistics -
func (srv *MyServer) Statistics(si *StatInterval, ass Admin_StatisticsServer) error {
	return nil
}

// Check -
func (srv *MyServer) Check(context.Context, *Nothing) (*Nothing, error) {
	return nil, nil
}

// Add -
func (srv *MyServer) Add(context.Context, *Nothing) (*Nothing, error) {
	return nil, nil
}

// Test -
func (srv *MyServer) Test(context.Context, *Nothing) (*Nothing, error) {
	return nil, nil
}

// StartMyMicroservice - entry point
func StartMyMicroservice(ctx context.Context, listenAddr, ACLData string) error {
	// parse input data

	lis, err := net.Listen("tcp", ":8082")
	if err != nil {
		log.Fatalln("can not listen port", err)
		return err
	}

	server := grpc.NewServer()

	// strater
	go func(listner net.Listener, gserver *grpc.Server) {
		mysrv := NewMyServer()
		RegisterAdminServer(server, mysrv)
		RegisterBizServer(server, mysrv)
		fmt.Println("starting server at :8082")
		gserver.Serve(listner)
	}(lis, server)

	// ender
	go func(ctx context.Context, gserver *grpc.Server) {
		<-ctx.Done()
		fmt.Println("stopping server at :8082")
		gserver.GracefulStop()
	}(ctx, server)

	return nil
}
