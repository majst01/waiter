package main

import (
	"context"
	"flag"
	"net"
	"time"

	v1 "github.com/metal-pod/waiter/api/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// WaitServer must implement wait services
type WaitServer struct {
	log *zap.Logger
	ac  chan v1.AllocRequest
}

func NewWaitServer(ac chan v1.AllocRequest, log *zap.Logger) *WaitServer {
	return &WaitServer{
		log: log,
		ac:  ac,
	}
}

// Wait implements the wait endpoint
func (s *WaitServer) Wait(req *v1.WaitRequest, srv v1.Wait_WaitServer) error {
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
		} else {
			s.log.Sugar().Infow("wait: resend alloc", "waiter id", req.Id, "id", ar.Id, "msg", ar.Message)
			s.ac <- ar
		}
	}
	return nil
}

func (s *WaitServer) Alloc(ctx context.Context, ar *v1.AllocRequest) (*v1.AllocResponse, error) {
	s.log.Sugar().Infow("alloc", "message", ar.Message)
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

	opts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 30 * time.Second,
		}),
	}
	grpcServer := grpc.NewServer(opts...)

	ac := make(chan v1.AllocRequest, 1)
	waitService := NewWaitServer(ac, log)

	v1.RegisterWaitServer(grpcServer, waitService)
	if err := grpcServer.Serve(lis); err != nil {
		log.Sugar().Fatalf("failed to serve: %v", err)
	}
}
