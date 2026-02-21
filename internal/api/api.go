package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type KdbResponse struct {
	Callback string          `json:"callback"`
	Result   json.RawMessage `json:"result"`
	Error    string          `json:"error"`
}

type KdbRequest struct {
	Callback string `json:"callback"`
	Cmd      string `json:"cmd"`
}

type ApiServer struct {
	HubPort    string
	ListenPort string
	conn       *websocket.Conn
	callbacks  sync.Map // map[string]chan KdbResponse
	mu         sync.Mutex
}

// Start now accepts certFile and keyFile and decides whether to use TLS
func Start(hubPort, listenPort, certFile, keyFile string) {
	s := &ApiServer{
		HubPort:    hubPort,
		ListenPort: listenPort,
	}

	// 1. Connect to kdb+ hub in background
	go s.connectToKdb()

	// 2. Setup Routes
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "API is alive") })
	mux.HandleFunc("/query", s.handleQuery)
	mux.HandleFunc("/up/", s.handleProcessAction("up", ".hub.up"))
	mux.HandleFunc("/down/", s.handleProcessAction("down", ".hub.down"))

	// 3. Wrap with CORS Middleware
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		mux.ServeHTTP(w, r)
	})

	// 4. Logic to determine if we should use SSL
	useTLS := false
	if certFile != "" && keyFile != "" {
		if _, err := os.Stat(certFile); err == nil {
			useTLS = true
		}
	}

	// 5. Start Redirector (only if we are actually using TLS on port 443)
	if useTLS && listenPort == "443" {
		go func() {
			log.Printf("↪️  Redirecting HTTP (:80) to HTTPS (:443)")
			http.ListenAndServe(":80", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "https://"+r.Host+r.RequestURI, http.StatusMovedPermanently)
			}))
		}()
	}

	// 6. Start the Server
	if useTLS {
		log.Printf("🚀 API listening on :%s (HTTPS/TLS enabled)", listenPort)
		log.Fatal(http.ListenAndServeTLS(":"+listenPort, certFile, keyFile, handler))
	} else {
		log.Printf("🚀 API listening on :%s (HTTP mode - No SSL found)", listenPort)
		log.Fatal(http.ListenAndServe(":"+listenPort, handler))
	}
}

func (s *ApiServer) connectToKdb() {
	url := fmt.Sprintf("ws://127.0.0.1:%s", s.HubPort)
	for {
		c, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			log.Printf("⚠️ Kdb connection failed: %v. Retrying...", err)
			time.Sleep(2 * time.Second)
			continue
		}
		s.conn = c
		log.Printf("✅ Connected to kdb hub at %s", url)
		s.readLoop()
	}
}

func (s *ApiServer) readLoop() {
	for {
		if s.conn == nil {
			time.Sleep(1 * time.Second)
			continue
		}
		_, message, err := s.conn.ReadMessage()
		if err != nil {
			log.Println("❌ Kdb read error:", err)
			s.conn = nil
			return
		}

		var resp KdbResponse
		if err := json.Unmarshal(message, &resp); err == nil {
			if ch, ok := s.callbacks.Load(resp.Callback); ok {
				ch.(chan KdbResponse) <- resp
			}
		}
	}
}

func (s *ApiServer) kdbQuery(cmd string) (json.RawMessage, error) {
	if s.conn == nil {
		return nil, fmt.Errorf("kdb hub not connected")
	}

	cbName := fmt.Sprintf("cb_%d", time.Now().UnixNano())
	ch := make(chan KdbResponse, 1)
	s.callbacks.Store(cbName, ch)
	defer s.callbacks.Delete(cbName)

	s.mu.Lock()
	err := s.conn.WriteJSON(KdbRequest{Callback: cbName, Cmd: cmd})
	s.mu.Unlock()

	if err != nil {
		return nil, err
	}

	select {
	case res := <-ch:
		if res.Error != "" {
			return nil, fmt.Errorf(res.Error)
		}
		return res.Result, nil
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("kdb timeout")
	}
}

func (s *ApiServer) handleQuery(w http.ResponseWriter, r *http.Request) {
	res, err := s.kdbQuery("4+8") // Example hardcoded query
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(res)
}

func (s *ApiServer) handleProcessAction(action, kdbFunc string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Path[len("/"+action+"/"):]
		_, err := s.kdbQuery(fmt.Sprintf("%s[`%s]", kdbFunc, name))
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		fmt.Fprintf(w, `{"status":"%sed", "process":"%s"}`, action, name)
	}
}
