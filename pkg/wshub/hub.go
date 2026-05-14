// Package wshub keeps the per-replica registry of open WebSocket clients.
// It is fed by notify.RedisBus.Subscribe in the API process — one mux
// channel in, fan-out to many clients out.
//
// Concurrency model: a single sync.RWMutex protects the client maps.
// Writes (subscribe/unsubscribe) are O(log n) on map ops; reads
// (broadcast) iterate, O(n_subscribers_for_user). For the small fan-out
// numbers we care about (a handful of admins, one or two devices per
// user) this is comfortably faster than channel-based actor patterns.
package wshub

import "sync"

// Client is the minimal interface the Hub needs. Decouples wshub from the
// websocket library so the Hub can be unit-tested with fake clients.
type Client interface {
	// Send pushes one envelope-bytes onto the client's send queue.
	// Implementations MUST be non-blocking and MUST drop on overflow.
	Send(payload []byte)

	// IsAdmin reports whether this client is subscribed to the admin
	// fan-out channel.
	IsAdmin() bool

	// UserID identifies the owner of this connection. Empty for guests
	// (which currently can't open a WS).
	UserID() string
}

// Hub tracks live WebSocket connections.
type Hub struct {
	mu sync.RWMutex
	// users maps user_id → set of clients (a user can have multiple
	// devices/tabs open). Map-of-maps for O(1) registration removal.
	users map[string]map[Client]struct{}
	// admins is the broadcast set for admin-only events.
	admins map[Client]struct{}
}

func NewHub() *Hub {
	return &Hub{
		users:  make(map[string]map[Client]struct{}),
		admins: make(map[Client]struct{}),
	}
}

// Register adds c to the user fan-out and, if c.IsAdmin(), also to the
// admin broadcast set.
func (h *Hub) Register(c Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if uid := c.UserID(); uid != "" {
		set, ok := h.users[uid]
		if !ok {
			set = make(map[Client]struct{}, 1)
			h.users[uid] = set
		}
		set[c] = struct{}{}
	}
	if c.IsAdmin() {
		h.admins[c] = struct{}{}
	}
}

// Unregister removes c from every set it was in. Safe to call multiple
// times — missing keys are no-ops.
func (h *Hub) Unregister(c Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if uid := c.UserID(); uid != "" {
		if set, ok := h.users[uid]; ok {
			delete(set, c)
			if len(set) == 0 {
				delete(h.users, uid)
			}
		}
	}
	delete(h.admins, c)
}

// SendToUser pushes payload to every client owned by userID. Empty
// userID is a no-op (guest jobs don't get notifications).
func (h *Hub) SendToUser(userID string, payload []byte) {
	if userID == "" {
		return
	}
	h.mu.RLock()
	set := h.users[userID]
	if len(set) == 0 {
		h.mu.RUnlock()
		return
	}
	// Snapshot under the lock so a concurrent unregister doesn't race
	// against the Send loop. Slice alloc is cheap; the map iteration
	// would otherwise need to hold the read lock for the whole fan-out.
	clients := make([]Client, 0, len(set))
	for c := range set {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	for _, c := range clients {
		c.Send(payload)
	}
}

// BroadcastAdmins pushes payload to every connected admin client.
func (h *Hub) BroadcastAdmins(payload []byte) {
	h.mu.RLock()
	if len(h.admins) == 0 {
		h.mu.RUnlock()
		return
	}
	clients := make([]Client, 0, len(h.admins))
	for c := range h.admins {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	for _, c := range clients {
		c.Send(payload)
	}
}

// Counts returns the current registry sizes — handy for /metrics or a
// debug endpoint. (totalUsers, totalConnections, totalAdmins).
func (h *Hub) Counts() (users, conns, admins int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	users = len(h.users)
	for _, set := range h.users {
		conns += len(set)
	}
	admins = len(h.admins)
	return
}
