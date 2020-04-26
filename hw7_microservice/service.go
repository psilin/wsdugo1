package main

import (
	"context"
	"encoding/json"
	fmt "fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// MyLogger -
type MyLogger struct {
	chev   chan Event
	finish context.CancelFunc
}

// MyStat -
type MyStat struct {
	byMethod   map[string]uint64
	byConsumer map[string]uint64
	finish     context.CancelFunc
}

// Add -
func (ms *MyStat) Add(consumer, method string) {
	if _, ok := ms.byConsumer[consumer]; !ok {
		ms.byConsumer[consumer] = 1
	} else {
		ms.byConsumer[consumer]++
	}

	if _, ok := ms.byMethod[method]; !ok {
		ms.byMethod[method] = 1
	} else {
		ms.byMethod[method]++
	}
}

// Reset -
func (ms *MyStat) Reset() {
	ms.byMethod = make(map[string]uint64)
	ms.byConsumer = make(map[string]uint64)
}

// MyServer -
type MyServer struct {
	mu   sync.RWMutex
	acl  map[string][]string
	myls []*MyLogger
	myss []*MyStat
	wg   *sync.WaitGroup
}

// NewMyServer -
func NewMyServer(ACLData map[string][]string) *MyServer {
	return &MyServer{
		mu:   sync.RWMutex{},
		acl:  ACLData,
		myls: []*MyLogger{},
		myss: []*MyStat{},
		wg:   &sync.WaitGroup{},
	}
}

// Logging -
func (srv *MyServer) Logging(n *Nothing, als Admin_LoggingServer) error {
	// logging
	md, ok := metadata.FromIncomingContext(als.Context())
	if !ok {
		return status.Errorf(codes.Unauthenticated, "Authorization failed")
	}

	e := Event{Consumer: md.Get("consumer")[0], Method: "/main.Admin/Logging", Host: "127.0.0.1:"}
	srv.mu.Lock()
	// add to stat
	for _, mys := range srv.myss {
		mys.Add(e.Consumer, e.Method)
	}

	for _, myl := range srv.myls {
		myl.chev <- e
	}
	srv.mu.Unlock()

	// add logger
	ctx, fin := context.WithCancel(context.Background())
	myl := MyLogger{chev: make(chan Event, 1), finish: fin}
	srv.wg.Add(1)
	srv.mu.Lock()
	srv.myls = append(srv.myls, &myl)
	srv.mu.Unlock()
LOOP:
	for {
		select {
		case e := <-myl.chev:
			err := als.Send(&e)
			if err != nil {
				fmt.Printf("LOGGING: %s\n", err)
			}
		case <-ctx.Done():
			break LOOP
		}
	}
	srv.wg.Done()
	return nil
}

// Statistics -
func (srv *MyServer) Statistics(si *StatInterval, ass Admin_StatisticsServer) error {
	// logging
	md, ok := metadata.FromIncomingContext(ass.Context())
	if !ok {
		return status.Errorf(codes.Unauthenticated, "Authorization failed")
	}

	e := Event{Consumer: md.Get("consumer")[0], Method: "/main.Admin/Statistics", Host: "127.0.0.1:"}
	srv.mu.Lock()
	// add to stat
	for _, mys := range srv.myss {
		mys.Add(e.Consumer, e.Method)
	}

	for _, myl := range srv.myls {
		myl.chev <- e
	}

	ctx, fin := context.WithCancel(context.Background())
	mys := MyStat{finish: fin, byConsumer: make(map[string]uint64), byMethod: make(map[string]uint64)}
	srv.myss = append(srv.myss, &mys)
	srv.mu.Unlock()

	srv.wg.Add(2)
	ticker := time.NewTicker(time.Duration(si.IntervalSeconds) * time.Second)
	inch := make(chan interface{}, 1)
	go func(t *time.Ticker, i chan interface{}, wgg *sync.WaitGroup) {
		for range t.C {
			inch <- 0
		}
		wgg.Done()
	}(ticker, inch, srv.wg)

LOOP:
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			break LOOP
		case <-inch:
			srv.mu.Lock()
			st := Stat{ByMethod: mys.byMethod, ByConsumer: mys.byConsumer}
			err := ass.Send(&st)
			if err != nil {
				fmt.Printf("STAT: %s\n", err)
			}
			mys.Reset()
			srv.mu.Unlock()
		}

	}
	srv.wg.Done()
	return nil
}

// Check -
func (srv *MyServer) Check(ctx context.Context, n *Nothing) (*Nothing, error) {
	// logging
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "Authorization failed")
	}

	e := Event{Consumer: md.Get("consumer")[0], Method: "/main.Biz/Check", Host: "127.0.0.1:"}
	srv.mu.Lock()
	// add to stat
	for _, mys := range srv.myss {
		mys.Add(e.Consumer, e.Method)
	}

	for _, myl := range srv.myls {
		myl.chev <- e
	}
	srv.mu.Unlock()
	return &Nothing{}, nil
}

// Add -
func (srv *MyServer) Add(ctx context.Context, n *Nothing) (*Nothing, error) {
	// logging
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "Authorization failed")
	}

	e := Event{Consumer: md.Get("consumer")[0], Method: "/main.Biz/Add", Host: "127.0.0.1:"}
	srv.mu.Lock()
	// add to stat
	for _, mys := range srv.myss {
		mys.Add(e.Consumer, e.Method)
	}

	for _, myl := range srv.myls {
		myl.chev <- e
	}
	srv.mu.Unlock()
	return &Nothing{}, nil
}

// Test -
func (srv *MyServer) Test(ctx context.Context, n *Nothing) (*Nothing, error) {
	// logging
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "Authorization failed")
	}

	e := Event{Consumer: md.Get("consumer")[0], Method: "/main.Biz/Test", Host: "127.0.0.1:"}
	srv.mu.Lock()
	// add to stat
	for _, mys := range srv.myss {
		mys.Add(e.Consumer, e.Method)
	}

	for _, myl := range srv.myls {
		myl.chev <- e
	}
	srv.mu.Unlock()
	return &Nothing{}, nil
}

func (srv *MyServer) authUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "Authorization failed")
	}

	consumers := md.Get("consumer")
	access := false
	for _, c := range consumers {
		if _, ok = srv.acl[c]; ok {
			for _, cc := range srv.acl[c] {
				if cc == info.FullMethod {
					access = true
				}
			}
		}
	}

	if !access {
		return nil, status.Errorf(codes.Unauthenticated, "Authorization failed")
	}

	reply, err := handler(ctx, req)

	return reply, err
}

func (srv *MyServer) authStreamInterceptor(ssrv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := ss.Context()
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Errorf(codes.Unauthenticated, "Authorization failed")
	}

	consumers := md.Get("consumer")
	access := false
	for _, c := range consumers {
		if _, ok = srv.acl[c]; ok {
			for _, cc := range srv.acl[c] {
				if cc == info.FullMethod {
					access = true
				}
			}
		}
	}

	if !access {
		return status.Errorf(codes.Unauthenticated, "Authorization failed")
	}
	return handler(ssrv, ss)
}

// StartMyMicroservice - entry point
func StartMyMicroservice(ctx context.Context, listenAddr, ACLData string) error {
	acl := make(map[string][]string)
	err := json.Unmarshal([]byte(ACLData), &acl)
	if err != nil {
		return err
	}

	// repack acl
	for i, v := range acl {
	INNER_LOOP_BIZ:
		for idx, vv := range v {
			if vv == "/main.Biz/*" {
				v[idx] = "/main.Biz/Add"
				acl[i] = append(acl[i], "/main.Biz/Check")
				acl[i] = append(acl[i], "/main.Biz/Test")
				break INNER_LOOP_BIZ
			}
		}

	INNER_LOOP_ADM:
		for idx, vv := range v {
			if vv == "/main.Admin/*" {
				v[idx] = "/main.Admin/Logging"
				acl[i] = append(acl[i], "/main.Admin/Statistics")
				break INNER_LOOP_ADM
			}
		}
	}

	port := strings.Split(listenAddr, ":")[1]
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalln("can not listen port", err)
		return err
	}

	//fmt.Printf("ACL: %v\n", acl)
	mysrv := NewMyServer(acl)
	server := grpc.NewServer(grpc.UnaryInterceptor(mysrv.authUnaryInterceptor), grpc.StreamInterceptor(mysrv.authStreamInterceptor))
	RegisterAdminServer(server, mysrv)
	RegisterBizServer(server, mysrv)

	// strater
	go func(listner net.Listener, gserver *grpc.Server) {
		//fmt.Println("starting server at :8082")
		gserver.Serve(listner)
	}(lis, server)

	// ender
	go func(ctx context.Context, gserver *grpc.Server, mysrv *MyServer) {
		<-ctx.Done()
		//fmt.Println("stopping server at :8082")
		// cancel loggers
		for _, myl := range mysrv.myls {
			myl.finish()
		}
		// cancel statters
		for _, mys := range mysrv.myss {
			mys.finish()
		}
		mysrv.wg.Wait()
		gserver.GracefulStop()
	}(ctx, server, mysrv)

	return nil
}
