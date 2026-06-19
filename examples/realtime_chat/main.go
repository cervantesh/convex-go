package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	convex "github.com/cervantesh/convex-go"
)

const (
	liveListMessagesPath = "live:listMessages"
	liveSendMessagePath  = "live:sendMessage"
)

type config struct {
	deploymentURL string
	authToken     string
	room          string
	send          string
	once          bool
}

type liveMessage struct {
	ID           string  `json:"_id"`
	CreationTime float64 `json:"_creationTime"`
	Room         string  `json:"room"`
	Body         string  `json:"body"`
	RequestID    string  `json:"requestId"`
}

func main() {
	log.SetFlags(0)
	if err := run(parseConfig()); err != nil {
		log.Fatal(err)
	}
}

func parseConfig() config {
	cfg := config{}
	flag.StringVar(&cfg.deploymentURL, "url", os.Getenv("CONVEX_URL"), "Convex deployment URL")
	flag.StringVar(&cfg.authToken, "token", os.Getenv("CONVEX_AUTH_TOKEN"), "Optional bearer auth token")
	flag.StringVar(&cfg.room, "room", "general", "Room name for live:listMessages and live:sendMessage")
	flag.StringVar(&cfg.send, "send", "", "Optional message body to send through live:sendMessage")
	flag.BoolVar(&cfg.once, "once", false, "Query once, optionally send one message, then exit without realtime streaming")
	flag.Parse()
	return cfg
}

func run(cfg config) error {
	if cfg.deploymentURL == "" {
		return fmt.Errorf("set CONVEX_URL or pass -url")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client, err := convex.NewClient(cfg.deploymentURL)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	if cfg.authToken != "" {
		client.SetAuth(cfg.authToken)
	}

	stopState := client.SubscribeToConnectionState(func(state convex.ConnectionState) {
		log.Printf("connection phase=%s retries=%d connected=%t", state.Phase, state.ConnectionRetries, state.HasEverConnected)
	})
	defer stopState()

	initial, err := listMessages(ctx, client, cfg.room)
	if err != nil {
		return err
	}
	if err := printMessages("initial", initial); err != nil {
		return err
	}

	if cfg.once {
		if cfg.send == "" {
			return nil
		}
		requestID, err := sendMessage(ctx, client, cfg.room, cfg.send)
		if err != nil {
			return err
		}
		log.Printf("sent request_id=%s", requestID)
		updated, err := listMessages(ctx, client, cfg.room)
		if err != nil {
			return err
		}
		return printMessages("after-send", updated)
	}

	subscription, err := client.Subscribe(ctx, liveListMessagesPath, map[string]any{"room": cfg.room})
	if err != nil {
		return err
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = subscription.Unsubscribe(cleanupCtx)
	}()

	if cfg.send != "" {
		requestID, err := sendMessage(ctx, client, cfg.room, cfg.send)
		if err != nil {
			return err
		}
		log.Printf("sent request_id=%s", requestID)
	}

	for {
		result, err := subscription.Next(ctx)
		switch {
		case errors.Is(err, context.Canceled), errors.Is(err, convex.ErrSubscriptionClosed):
			return nil
		case err != nil:
			return err
		}
		if err := result.Err(); err != nil {
			return err
		}
		value, ok := result.Value()
		if !ok {
			continue
		}
		messages, err := decodeMessages(value.GoValue())
		if err != nil {
			return err
		}
		if err := printMessages("realtime", messages); err != nil {
			return err
		}
	}
}

func listMessages(ctx context.Context, client *convex.Client, room string) ([]liveMessage, error) {
	var messages []liveMessage
	if err := client.QueryInto(ctx, liveListMessagesPath, map[string]any{"room": room}, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func sendMessage(ctx context.Context, client *convex.Client, room, body string) (string, error) {
	requestID := fmt.Sprintf("go-demo-%d", time.Now().UnixNano())
	_, err := client.Mutation(ctx, liveSendMessagePath, map[string]any{
		"room":      room,
		"body":      body,
		"requestId": requestID,
	})
	if err != nil {
		return "", err
	}
	return requestID, nil
}

func decodeMessages(input any) ([]liveMessage, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var messages []liveMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func printMessages(label string, messages []liveMessage) error {
	payload := struct {
		Label    string        `json:"label"`
		Messages []liveMessage `json:"messages"`
	}{
		Label:    label,
		Messages: messages,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}
