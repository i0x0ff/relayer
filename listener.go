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

func setListener(id string, ws *Connection, filters nostr.Filters) {
	listenersMutex.Lock()
	defer listenersMutex.Unlock()

	subs, ok := listeners[ws]
	if !ok {
		subs = make(map[string]*Listener)
		listeners[ws] = subs
	}

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
			delete(listeners, ws)
		}
	}
}

// Remove WebSocket conn from listeners
func removeListener(ws *Connection) {
	listenersMutex.Lock()
	defer listenersMutex.Unlock()

	_, ok := listeners[ws]
	if ok {
		delete(listeners, ws)
	}
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
