package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	v1 "github.com/metal-pod/waiter/api/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Client must implement wait services
type Client struct {
	c   *grpc.ClientConn
	log *zap.Logger
}

func NewClient(address string, logger *zap.Logger) (*Client, error) {
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
	}

	// Set up a connection to the server.
	conn, err := grpc.Dial(address, opts...)
	if err != nil {
		logger.Sugar().Errorf("did not connect: %v", err)
		return nil, err
	}

	return &Client{
		c:   conn,
		log: logger,
	}, nil
}

func (c *Client) Close() error {
	return c.c.Close()
}

func (c *Client) Wait(id string) error {
	w := v1.NewWaitClient(c.c)
	wr := &v1.WaitRequest{
		Id: id,
	}
	wc, err := w.Wait(context.Background(), wr)
	if err != nil {
		return err
	}
	for {
		resp, err := wc.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			c.log.Sugar().Errorw("error while waiting, retry in 2sec", "err", err)
			time.Sleep(time.Second * 2)
		}
		if resp != nil {
			c.log.Sugar().Infow("wait response", "id", resp.Id, "message", resp.Message)
		} else {
			c.log.Sugar().Error("got nil response")
		}
	}
	return fmt.Errorf("no wait response received")
}

func (c *Client) Alloc(id, message string) error {
	a := v1.NewWaitClient(c.c)
	ar := &v1.AllocRequest{
		Id:      id,
		Message: message,
	}
	_, err := a.Alloc(context.Background(), ar)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	var address = flag.String("address", "localhost:50051", "address to connect")
	var alloc = flag.String("alloc", "", "alloc")
	var id = flag.String("id", "", "id")
	flag.Parse()

	log, _ := zap.NewProduction()
	c, err := NewClient(*address, log)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer c.Close()

	if id == nil {
		log.Fatal("id required")
	}
	if alloc != nil && *alloc != "" {
		c.Alloc(*id, *alloc)
		os.Exit(0)
	}
	for {
		err := c.Wait(*id)
		if err != nil {
			log.Sugar().Errorw("error connecting to server, retrying in 2sec", "err", err)
			time.Sleep(time.Second * 2)
		}
	}
}
