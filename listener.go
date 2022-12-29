package relayer

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/nbd-wtf/go-nostr"
)

type Listener struct {
	filters nostr.Filters
}

var listeners = make(map[*WebSocket]map[string]*Listener)
var listenersMutex = sync.Mutex{}

func GetListenerCount() {
	count := len(listeners)
	fmt.Println("Current number of listeners:", count)
}

func GetListeningFilters() nostr.Filters {
	var respfilters = make(nostr.Filters, 0, len(listeners)*2)

	listenersMutex.Lock()
	defer func() {
		listenersMutex.Unlock()
	}()

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

func setListener(id string, ws *WebSocket, filters nostr.Filters, r *http.Request) {
	listenersMutex.Lock()
	defer func() {
		listenersMutex.Unlock()
	}()

	subs, ok := listeners[ws]
	if !ok {
		subs = make(map[string]*Listener)
		listeners[ws] = subs
	}

	subs_count_before := len(listeners[ws])

	_, ok = subs[id]
	if !ok {
		fmt.Printf("Sub id=%s doesn't exist yet - add it\n", id)
	} else {
		fmt.Printf("Sub id=%s already eixsts - update it\n", id)
	}

	subs[id] = &Listener{
		filters: filters,
	}

	fmt.Printf("Subs for %s increased from %d to %d\n", r.Header.Get("X-FORWARDED-FOR"), subs_count_before, len(listeners[ws]))
}

// Remove a specific subscription id from listeners for a given ws client
func removeListenerId(ws *WebSocket, id string) {
	listenersMutex.Lock()
	defer func() {
		listenersMutex.Unlock()
	}()

	subs, ok := listeners[ws]
	if ok {
		delete(listeners[ws], id)
		if len(subs) == 0 {
			delete(listeners, ws)
		}
	}
}

// Remove WebSocket conn from listeners
func removeListener(ws *WebSocket) {
	listenersMutex.Lock()
	defer listenersMutex.Unlock()

	_, ok := listeners[ws]
	if ok {
		delete(listeners, ws)
	}
}

func notifyListeners(event *nostr.Event) {
	listenersMutex.Lock()
	defer func() {
		listenersMutex.Unlock()
	}()

	for ws, subs := range listeners {
		for id, listener := range subs {
			if !listener.filters.Match(event) {
				continue
			}
			ws.WriteJSON([]interface{}{"EVENT", id, event})
		}
	}
}
