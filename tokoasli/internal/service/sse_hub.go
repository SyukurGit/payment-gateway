package service

import (
	"encoding/json"
	"sync"
)

type SSEHub struct {
	clients map[string]map[chan []byte]bool
	mu      sync.RWMutex
}

func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients: make(map[string]map[chan []byte]bool),
	}
}

// Subscribe registers a client channel for a specific order ID
// Returns the event channel and a cleanup function to unsubscribe
func (h *SSEHub) Subscribe(orderID string) (chan []byte, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan []byte, 10)
	if _, exists := h.clients[orderID]; !exists {
		h.clients[orderID] = make(map[chan []byte]bool)
	}
	h.clients[orderID][ch] = true

	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		if subs, exists := h.clients[orderID]; exists {
			delete(subs, ch)
			close(ch)
			if len(subs) == 0 {
				delete(h.clients, orderID)
			}
		}
	}

	return ch, unsubscribe
}

// Broadcast sends JSON data to all active clients watching orderID
func (h *SSEHub) Broadcast(orderID string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	if subs, exists := h.clients[orderID]; exists {
		for ch := range subs {
			select {
			case ch <- data:
			default:
				// If channel buffer is full, drop to prevent blocking
			}
		}
	}
}
