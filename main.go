package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ascii8/nakama-go"
)

func main() {
	urlstr := flag.String("url", "http://127.0.0.1:7350", "url")
	key := flag.String("key", "xoxo-go_server", "server key")
	d := flag.Duration("d", 10*time.Minute, "duration")
	userId := flag.String("id", "d2bb1a95-5f68-4903-b8ba-77eeebed363e", "user id")
	username := flag.String("username", "username", "username")
	flag.Parse()
	if err := run(context.Background(), *urlstr, *key, *d, *userId, *username); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, urlstr, key string, d time.Duration, userId, username string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	sigCh := make(chan bool)
	go func() {
		defer close(sigCh)
		// catch signals, canceling context to cause cleanup
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		var last time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case sig := <-ch:
				log.Printf("caught signal: %v", sig)
				if sig == syscall.SIGINT {
					t := time.Now()
					if !last.IsZero() && t.Sub(last) < 1*time.Second {
						log.Printf("caught %v twice, exiting", sig)
						cancel()
						return
					}
					sigCh <- true
					last = t
				} else {
					cancel()
					return
				}
			}
		}
	}()
	m := newClient(userId, username)
	cl := nakama.New(
		nakama.WithURL(urlstr),
		nakama.WithServerKey(key),
		nakama.WithLogger(log.Printf),
		nakama.WithAuthHandler(m),
	)
	conn, err := cl.NewConn(
		ctx,
		nakama.WithConnFormat("json"),
		nakama.WithConnHandler(m),
		nakama.WithConnPersist(true),
	)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(d):
			return nil
		case t := <-sigCh:
			if !t {
				return nil
			}
			if err := conn.CloseWithStopErr(false, nil); err != nil {
				return err
			}
		}
	}
}

type Client struct {
	userId   string
	username string
}

func newClient(id, username string) *Client {
	return &Client{
		userId:   id,
		username: username,
	}
}

func (cl *Client) AuthHandler(ctx context.Context, nakamaClient *nakama.Client) error {
	log.Printf("authenticating")
	return nakamaClient.AuthenticateDevice(ctx, cl.userId, true, cl.username)
}

func (cl *Client) ConnectHandler(context.Context) {
	log.Printf("connected!")
}

func (cl *Client) DisconnectHandler(_ context.Context, err error) {
	log.Printf("disconnect: %v", err)
}
