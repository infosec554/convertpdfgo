package notify

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"

	"convertpdfgo/config"
)

// Channel naming. Kept short — every published message pays the channel
// name in network bytes.
const (
	channelUserPrefix = "n:u:"  // "n:u:<user_id>"
	channelAdmin      = "n:a"   // single admin fanout channel
	channelMux        = "n:mux" // multiplexed channel that the API replica subscribes to once
)

// muxMessage is what flows over Redis pub/sub. Audience routes the
// envelope at the subscriber side to either the per-user fanout or the
// admin broadcast — one Redis subscription instead of N+1.
type muxMessage struct {
	Audience string   `json:"a"` // "u" (user) or "a" (admin)
	UserID   string   `json:"u,omitempty"`
	Env      Envelope `json:"e"`
}

// RedisBus publishes events the worker process emits so the API process
// can fan them out to WebSocket clients. Both sides instantiate one of
// these from the same Redis config.
type RedisBus struct {
	client *redis.Client
}

// NewRedisBus opens a dedicated client (don't share with the cache layer
// — pub/sub holds the connection open and would block normal commands).
func NewRedisBus(cfg config.Config) *RedisBus {
	return &RedisBus{
		client: redis.NewClient(&redis.Options{
			Addr:     cfg.RedisHost + ":" + cfg.RedisPort,
			Password: cfg.RedisPassword,
			DB:       0,
		}),
	}
}

func (r *RedisBus) Close() error { return r.client.Close() }

func (r *RedisBus) PublishUser(ctx context.Context, userID string, e Envelope) error {
	if userID == "" {
		return nil
	}
	return r.publish(ctx, muxMessage{Audience: "u", UserID: userID, Env: e})
}

func (r *RedisBus) PublishAdmin(ctx context.Context, e Envelope) error {
	return r.publish(ctx, muxMessage{Audience: "a", Env: e})
}

func (r *RedisBus) publish(ctx context.Context, m muxMessage) error {
	raw, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("notify: marshal mux: %w", err)
	}
	return r.client.Publish(ctx, channelMux, raw).Err()
}

// HandlerFunc is the callback Subscribe invokes for each received event.
// audience: "u" or "a". userID: present iff audience == "u".
type HandlerFunc func(audience, userID string, e Envelope)

// Subscribe blocks until ctx is cancelled, dispatching every message
// from the mux channel through h. Run this in its own goroutine inside
// the API process; pair it with the in-process Hub.
//
// We don't reconnect on transient Redis errors — go-redis already does
// internal reconnect; the iterator surfaces only fatal close. Returning
// the error lets the caller log + maybe restart the goroutine.
func (r *RedisBus) Subscribe(ctx context.Context, h HandlerFunc) error {
	ps := r.client.Subscribe(ctx, channelMux)
	defer ps.Close()

	ch := ps.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			var m muxMessage
			if err := json.Unmarshal([]byte(msg.Payload), &m); err != nil {
				continue // malformed event — drop and keep listening
			}
			h(m.Audience, m.UserID, m.Env)
		}
	}
}
