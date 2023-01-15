package nostrelay

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rs/cors"
	"github.com/stackerstan/go-nostr"
	"mindmachine/mindmachine"
)

type Instructions struct {
	List     []string
	Mind     string
	Sequence int64
}

var router = mux.NewRouter()

func Start() {
	mindmachine.LogCLI("Starting our local Nostr Relay for the frontend", 4)
	// catch the websocket call before anything else
	router.Path("/").Headers("Upgrade", "websocket").HandlerFunc(handleWebsocket())

	srv := &http.Server{
		Handler:           cors.Default().Handler(router),
		Addr:              mindmachine.MakeOrGetConfig().GetString("websocketAddr"),
		WriteTimeout:      2 * time.Second,
		ReadTimeout:       2 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}
	mindmachine.LogCLI(fmt.Sprintf("listening on "+srv.Addr), 4)
	err := srv.ListenAndServe()
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
}

const (
	//todo put this in the config instead

	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = pongWait / 2

	// Maximum message size allowed from peer.
	maxMessageSize = 5242880
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func PublishEvent(event nostr.Event) {
	if _, err := event.CheckSignature(); err == nil {
		currentState.upsert(event)
		//fmt.Printf("\n107 message size in bytes\n%d\n", len(event.Serialize()))
		if !startedRelays {
			go startRelaysForPublishing()
			startedRelays = true
		}
		go func() { publishQueue <- event }()
	} else {
		mindmachine.LogCLI("invalid signature on event "+event.ID, 2)
	}
}

var startedRelays = false
var publishQueue = make(chan nostr.Event)

func startRelaysForPublishing() {
	mindmachine.PruneDeadOptionalRelays()
	relays := mindmachine.MakeOrGetConfig().GetStringSlice("relaysMust")
	relays = append(relays, mindmachine.MakeOrGetConfig().GetStringSlice("relaysOptional")...)
	//pool := initRelays(relays)
	pool := nostr.NewRelayPool()
	mindmachine.LogCLI("Connecting to relay pool", 3)
	for _, s := range relays {
		errchan := pool.Add(s, nostr.SimplePolicy{Read: true, Write: true})
		go func() {
			for err := range errchan {
				e := fmt.Sprintf("j49fk: %s", err.Error())
				mindmachine.LogCLI(e, 2)
			}
		}()
	}
	defer func() {
		for _, s := range mindmachine.MakeOrGetConfig().GetStringSlice("relaysMust") {
			pool.Remove(s)
		}
	}()
	for {
		select {
		case event := <-publishQueue:
			e, _, err := pool.PublishEvent(&event)
			time.Sleep(time.Second) //don't spam relays
			//fmt.Printf("\n116\n%#v\n", &event)
			if err != nil {
				fmt.Printf("\n%#v\n", e)
				mindmachine.LogCLI("failed to publish an event, see event above", 1)
			}
		}
	}
}

//handleWebsocket handles connections from the user interfarce
func handleWebsocket() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		//mindmachine.LogCLI(fmt.Sprintf("HTTP connection from: %s", r.RemoteAddr), 4)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			mindmachine.LogCLI("failed to upgrade websocket", 3)
			return
		}
		ticker := time.NewTicker(pingPeriod)

		ws := &WebSocket{conn: conn}

		// reader
		go func() {
			defer func() {
				ticker.Stop()
				conn.Close()
			}()

			conn.SetReadLimit(maxMessageSize)
			conn.SetReadDeadline(time.Now().Add(pongWait))
			conn.SetPongHandler(func(string) error {
				conn.SetReadDeadline(time.Now().Add(pongWait))
				return nil
			})
			//mindmachine.LogCLI(fmt.Sprintf("WS connection established: %s", ws.conn.RemoteAddr().String()), 4)
			for {
				typ, message, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(
						err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						mindmachine.LogCLI("unexpected close of websocket", 3)
					}
					break
				}
				if typ == websocket.PingMessage {
					ws.WriteMessage(websocket.PongMessage, nil)
					continue
				}
				go func(message []byte) {
					var clientErrorResp string
					defer func() {
						if clientErrorResp != "" {
							ws.WriteJSON([]interface{}{"ERROR", clientErrorResp})
						}
					}()

					var request []json.RawMessage
					if err := json.Unmarshal(message, &request); err != nil {
						// stop silently
						return
					}

					if len(request) < 2 {
						clientErrorResp = "request has less than 2 parameters"
						return
					}

					var typ string
					json.Unmarshal(request[0], &typ)

					switch typ {
					case "EVENT":
						// we've received a nostr event
						var evt nostr.Event
						if err := json.Unmarshal(request[1], &evt); err != nil {
							mindmachine.LogCLI("failed to unmarshal event "+err.Error(), 3)
							return
						}
						//fmt.Printf("Kind: %d\n", evt.Kind)
						//fmt.Printf("Tags: %s\n", evt.Tags)
						//fmt.Printf("%#v", evt.Event)

						// check serialization
						serialized := evt.Serialize()

						// assign ID
						hash := sha256.Sum256(serialized)
						evt.ID = hex.EncodeToString(hash[:])

						// validate signature
						if ok, err := evt.CheckSignature(); err != nil {
							clientErrorResp = "signature verification error"
							return
						} else if !ok {
							clientErrorResp = "signature invalid"
							return
						}

						e := mindmachine.ConvertToInternalEvent(&evt)

						mindmachine.LogCLI("sending event to router", 4)
						sendToRouter <- e
						break
					case "REQ":
						var s string
						err := json.Unmarshal(request[1], &s)
						if err != nil {
							mindmachine.LogCLI(err.Error(), 3)
							clientErrorResp = err.Error()
							return
						}
						if s == "" {
							clientErrorResp = "REQ has no <id>"
							mindmachine.LogCLI("REQ has no <id>", 3)
							return
						}
						filters := make(nostr.Filters, len(request)-2)
						//fmt.Printf("\n180\n%#v\n", filters)
						//fmt.Printf("\n181\n%s\n", request)
						//fmt.Printf("\n182\n%s\n", request[2:])
						for i, filterReq := range request[2:] {
							//fmt.Printf("\n183\n%#v\n", filterReq)
							if err := json.Unmarshal(
								filterReq,
								&filters[i],
							); err != nil {
								clientErrorResp = "failed to decode filter"
								mindmachine.LogCLI("failed to decode filter", 3)
								return
							}
						}
						sub := Subscription{
							Filters:   filters,
							Events:    make(chan nostr.Event),
							Terminate: make(chan bool),
						}
						//todo if it has an e tag, forward to relays and proxy back the results
						//if etag, eTagExists := sub.Filters[0].Tags["e"]; eTagExists {
						//	go batchETagProxyRequests(etag, sub.Events)
						//}

					Mind:
						for mind, c := range subscriptions {
							for _, filter := range sub.Filters {
								for s2, _ := range filter.Tags {
									if s2 == mind {
										c <- sub
										continue Mind
									}
								}
							}
						}
						go func(sub Subscription) {
							for {
								event := <-sub.Events
								err := ws.WriteJSON([]interface{}{"EVENT", s, event})
								if err != nil {
									mindmachine.LogCLI(err.Error(), 2)
								}
							}
						}(sub)

						//testEvent := &nostr.Event{
						//	ID:        "863daeb2134b753e54aecd11c4aa8a49b5e4e259c0831e458d538ae382b71093",
						//	PubKey:    "22b583f571722012695faeba269c64255a6cb11440a08dfc3a7299a8b7c59e49",
						//	CreatedAt: time.UnixMilli(1652477967),
						//	Kind:      1,
						//	Tags:      nil,
						//	Content:   "TEST2",
						//	Sig:       "6b5f1df7cfb7007b7c09b03288ec4722b3aa948116c637234d1976be2c1a5e046917854ab1a8042360738ccc1829cd103740cceb2832985ee4e1da7392c3c0e9",
						//}
						//events, err := //todo get swarm objects from mindmachine state machines, and/or usual nostr events from relay //relay.QueryEvents(&filters[i])
						//if err == nil {
						//for _, event := range events {
						//ws.WriteJSON([]interface{}{"EVENT", id, testEvent})
						//}
						//}
						//}

						setListener(s, ws, filters)
						break
					case "CLOSE":
						var id string
						json.Unmarshal(request[0], &id)
						if id == "" {
							clientErrorResp = "CLOSE has no <id>"
							return
						}

						removeListener(ws, id)
						break
					default:
						clientErrorResp = "unknown message type " + typ
						return
					}
				}(message)
			}
		}()

		// writer
		go func() {
			defer func() {
				ticker.Stop()
				conn.Close()
			}()

			for {
				select {
				case <-ticker.C:
					err := ws.WriteMessage(websocket.PingMessage, nil)
					if err != nil {
						mindmachine.LogCLI("couldn't ping, exterminating socket", 3)
						return
					}
				}
			}
		}()
	}
}

//InjectEvent takes an event and sends it to the router, injecting it into the stream of events received over websockets
func InjectEvent(e nostr.Event) {
	var evt mindmachine.Event
	currentState.upsert(e)
	evt = mindmachine.ConvertToInternalEvent(&e)
	if ok, err := evt.CheckSignature(); ok {
		sendToRouter <- evt
	} else {
		mindmachine.LogCLI(err.Error(), 2)
	}
}

type Listener struct {
	filters nostr.Filters
}

var listeners = make(map[*WebSocket]map[string]*Listener)
var listenersMutex = sync.Mutex{}

func setListener(id string, ws *WebSocket, filters nostr.Filters) {
	listenersMutex.Lock()
	defer func() {
		listenersMutex.Unlock()
	}()

	subs, ok := listeners[ws]
	if !ok {
		subs = make(map[string]*Listener)
		listeners[ws] = subs
	}

	subs[id] = &Listener{
		filters: filters,
	}
}

func removeListener(ws *WebSocket, id string) {
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

var sendToRouter = make(chan mindmachine.Event, 1000)

// SubscribeToMessages returns a channel which
func SubscribeToMessages() chan mindmachine.Event {
	return sendToRouter
}

type Subscription struct {
	Filters   nostr.Filters
	Events    chan nostr.Event
	Terminate chan bool //close this to stop watching for new events
}

var subscriptions = make(map[string]chan Subscription)

func SubscribeToRequests(mind string) chan Subscription {
	subscriptions[mind] = make(chan Subscription)
	return subscriptions[mind]
}
