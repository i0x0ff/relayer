package relayer

import (
	"net/http"
	"sync"

	"github.com/nbd-wtf/go-nostr"
)

type Listener struct {
	filters nostr.Filters
}

type Connection struct {
	conn       *WebSocket
	httpHeader *http.Header
}

var listeners = make(map[*Connection]map[string]*Listener)
var listenersMutex = sync.Mutex{}

func GetListeningFilters() nostr.Filters {
	var respfilters = make(nostr.Filters, 0, len(listeners)*2)

	listenersMutex.Lock()
	defer listenersMutex.Unlock()

	// here we go through all the existing listeners
	for _, connlisteners := range listeners {
		for _, listener := range connlisteners {
			for _, listenerfilter := range listener.filters {
				for _, respfilter := range respfilters {
					// check if this filter specifically is already added to respfilters
					if nostr.FilterEqual(listenerfilter, respfilter) {
						goto nextconn
					}
				}

				// field not yet present on respfilters, add it
				respfilters = append(respfilters, listenerfilter)

				// continue to the next filter
			nextconn:
				continue
			}
		}
	}

	// respfilters will be a slice with all the distinct filter we currently have active
	return respfilters
}

func subsLimitReached(ws *Connection) bool {
	listenersMutex.Lock()
	defer listenersMutex.Unlock()
	subs := listeners[ws]
	return len(subs) >= 8
}

func setListener(id string, ws *Connection, filters nostr.Filters) {
	listenersMutex.Lock()
	defer listenersMutex.Unlock()

	// ip := ws.httpHeader.Get("X-FORWARDED-FOR")

	// fmt.Printf("[setListener] (%s) before count: %d\n", ip, len(listeners))
	subs, ok := listeners[ws]
	if !ok {
		subs = make(map[string]*Listener)
		listeners[ws] = subs
	}

	// fmt.Printf("[setListener] (%s) after count: %d\n", ip, len(listeners))
	// fmt.Printf("[setListener] (%s) subs: %d\n", ip, len(subs))

	subs[id] = &Listener{
		filters: filters,
	}
}

// Remove a specific subscription id from listeners for a given ws client
func removeListenerId(ws *Connection, id string) {
	listenersMutex.Lock()
	defer listenersMutex.Unlock()

	subs, ok := listeners[ws]
	if ok {
		delete(listeners[ws], id)
		if len(subs) == 0 {
			// fmt.Println("[removeListenerId] before count", len(listeners))
			delete(listeners, ws)
			// fmt.Println("[removeListenerId] after count", len(listeners))
		}
	}
}

// Remove WebSocket conn from listeners
func removeListener(ws *Connection) {
	listenersMutex.Lock()
	defer listenersMutex.Unlock()

	// fmt.Println("[removeListener] before count", len(listeners))

	_, ok := listeners[ws]
	if ok {
		delete(listeners, ws)
	}

	// fmt.Println("[removeListener] after count", len(listeners))
}

func notifyListeners(event *nostr.Event) {
	listenersMutex.Lock()
	defer listenersMutex.Unlock()

	for ws, subs := range listeners {
		for id, listener := range subs {
			if !listener.filters.Match(event) {
				continue
			}
			ws.conn.WriteJSON([]interface{}{"EVENT", id, event})
		}
	}
}

type ListenerStat struct {
	Ip            string `json:"ip"`
	Origin        string `json:"origin"`
	Subscriptions int    `json:"subscriptions"`
}

func ListenerStats() []ListenerStat {
	listenersMutex.Lock()
	defer listenersMutex.Unlock()

	var listenerStat []ListenerStat
	for conn, subs := range listeners {
		subs := len(subs)
		stat := ListenerStat{
			Ip:            conn.httpHeader.Get("X-FORWARDED-FOR"),
			Origin:        conn.httpHeader.Get("Origin"),
			Subscriptions: subs,
		}
		listenerStat = append(listenerStat, stat)
	}

	return listenerStat
}
