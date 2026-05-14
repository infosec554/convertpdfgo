package wshub

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

// sendQueueDepth is the per-client buffer between the Hub fan-out and
// the actual websocket write. A slow / disconnected client must not
// back up producers (worker, admin events). 64 is enough that a one-off
// burst (e.g. several jobs finishing at once) never drops; larger and
// we'd be wasting memory on idle connections.
const sendQueueDepth = 64

// writeTimeout caps how long the write goroutine will wait for a single
// frame to flush. If the network is dead this becomes the upper bound
// on detection latency.
const writeTimeout = 10 * time.Second

// pingInterval is how often the server sends a ping. Browsers react to
// ping/pong on the wire; without it, an idle TCP connection on a NAT
// box will silently die after the idle timer expires.
const pingInterval = 30 * time.Second

// WSClient adapts a *websocket.Conn into the wshub.Client interface.
// One instance per open browser connection.
type WSClient struct {
	conn    *websocket.Conn
	send    chan []byte
	userID  string
	isAdmin bool

	// closed is set atomically the first time we tear down so concurrent
	// Send / write-pump shutdowns are race-free.
	closed atomic.Bool
}

// NewClient wraps an already-accepted websocket connection.
func NewClient(conn *websocket.Conn, userID string, isAdmin bool) *WSClient {
	return &WSClient{
		conn:    conn,
		send:    make(chan []byte, sendQueueDepth),
		userID:  userID,
		isAdmin: isAdmin,
	}
}

func (c *WSClient) UserID() string { return c.userID }
func (c *WSClient) IsAdmin() bool  { return c.isAdmin }

// Send is the Hub's entry point. Drops on overflow — back-pressure on
// the producer is unacceptable for a notification system.
func (c *WSClient) Send(payload []byte) {
	if c.closed.Load() {
		return
	}
	select {
	case c.send <- payload:
	default:
		// Queue full. Client is too slow; close so the read loop notices
		// and unregisters. Better to drop the connection than to
		// silently drop messages forever.
		c.shutdown()
	}
}

// Run drives both pumps until the connection breaks. Caller is
// responsible for Hub.Register / Hub.Unregister around this call.
//
// readPump's only job is to detect close — we don't accept any client
// messages today, so we just drain and watch for errors. Without an
// active reader the connection's pong handler never fires and pings
// can't keep the link alive.
func (c *WSClient) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer c.shutdown()

	go c.readPump(ctx, cancel)
	c.writePump(ctx)
}

func (c *WSClient) readPump(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()
	for {
		// CloseRead returns a context that's cancelled when the peer
		// sends close — but we want to also drain any incoming frames
		// to keep the pong path alive. Read with no buffer reads + drops.
		_, _, err := c.conn.Read(ctx)
		if err != nil {
			return
		}
	}
}

func (c *WSClient) writePump(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case payload, ok := <-c.send:
			if !ok {
				return
			}
			wctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Write(wctx, websocket.MessageText, payload)
			cancel()
			if err != nil {
				return
			}
		case <-ticker.C:
			pctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Ping(pctx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (c *WSClient) shutdown() {
	if !c.closed.CompareAndSwap(false, true) {
		return
	}
	// Best-effort polite close. Errors are expected if the peer is
	// already gone; ignore.
	_ = c.conn.Close(websocket.StatusNormalClosure, "")
}

// IsClosedConnError reports whether err came from the peer disconnecting
// vs. a real protocol error worth logging.
func IsClosedConnError(err error) bool {
	if err == nil {
		return false
	}
	var ce websocket.CloseError
	if errors.As(err, &ce) {
		return true
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
