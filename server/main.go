package main

import (
	"context"
	"encoding/json"
	"flag"
	"net"
	"time"

	nsq "github.com/nsqio/go-nsq"

	v1 "github.com/metal-pod/waiter/api/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// WaitServer must implement wait services
type WaitServer struct {
	log *zap.Logger
	ac  chan v1.AllocRequest
	n   *nsq.Producer
}

func NewWaitServer(ac chan v1.AllocRequest, log *zap.Logger) *WaitServer {
	config := nsq.NewConfig()
	w, _ := nsq.NewProducer("127.0.0.1:4150", config)
	return &WaitServer{
		log: log,
		ac:  ac,
		n:   w,
	}
}

// Wait implements the wait endpoint
func (s *WaitServer) Wait(req *v1.WaitRequest, srv v1.Wait_WaitServer) error {
	wait, err := json.Marshal(req)
	if err != nil {
		return err
	}
	err = s.n.Publish("waiting", wait)
	if err != nil {
		return err
	}
	for ar := range s.ac {
		wr := &v1.WaitResponse{
			Id:      ar.Id,
			Message: ar.Message,
		}
		s.log.Sugar().Infow("wait: got alloc", "waiter id", req.Id, "id", ar.Id, "msg", ar.Message)
		if req.Id == ar.Id {
			err := srv.Send(wr)
			if err != nil {
				s.log.Sugar().Errorw("wait: unable to send", "err", err)
			}
		}

		s.log.Sugar().Infow("wait: ignore alloc", "waiter id", req.Id, "id", ar.Id, "msg", ar.Message)

	}
	return nil
}

func (s *WaitServer) Alloc(ctx context.Context, ar *v1.AllocRequest) (*v1.AllocResponse, error) {
	req, err := json.Marshal(ar)
	if err != nil {
		return nil, err
	}

	err = s.n.Publish("allocation", req)
	if err != nil {
		return nil, err
	}

	s.log.Sugar().Infow("alloc", "id", ar.Id, "message", ar.Message)
	s.ac <- *ar
	return &v1.AllocResponse{}, nil
}

func main() {
	log, _ := zap.NewProduction()
	var port = flag.String("port", "50051", "port to listen")
	flag.Parse()

	lis, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		log.Sugar().Fatalf("failed to listen: %v", err)
	}
	log.Sugar().Infof("listening on %s", *port)

	kaep := keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second, // If a client pings more than once every 5 seconds, terminate the connection
		PermitWithoutStream: true,            // Allow pings even when there are no active streams
	}

	kasp := keepalive.ServerParameters{
		MaxConnectionIdle:     15 * time.Second, // If a client is idle for 15 seconds, send a GOAWAY
		MaxConnectionAge:      30 * time.Second, // If any connection is alive for more than 30 seconds, send a GOAWAY
		MaxConnectionAgeGrace: 5 * time.Second,  // Allow 5 seconds for pending RPCs to complete before forcibly closing connections
		Time:                  5 * time.Second,  // Ping the client if it is idle for 5 seconds to ensure the connection is still active
		Timeout:               1 * time.Second,  // Wait 1 second for the ping ack before assuming the connection is dead
	}

	opts := []grpc.ServerOption{
		grpc.KeepaliveEnforcementPolicy(kaep),
		grpc.KeepaliveParams(kasp),
	}
	grpcServer := grpc.NewServer(opts...)

	ac := make(chan v1.AllocRequest, 1)
	waitService := NewWaitServer(ac, log)

	v1.RegisterWaitServer(grpcServer, waitService)
	if err := grpcServer.Serve(lis); err != nil {
		log.Sugar().Fatalf("failed to serve: %v", err)
	}
}
