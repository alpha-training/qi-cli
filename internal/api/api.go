package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
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
	HubPort     string
	ListenPort  string
	conn        *websocket.Conn
	callbacks   sync.Map // map[string]chan KdbResponse
	subscribers sync.Map // map[chan []byte]struct{}
	mu          sync.Mutex
}

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
	mux.HandleFunc("/stream", s.handleStream)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/liststacks/", s.handleListStacks)
	mux.HandleFunc("/readstack/", s.handleReadStack)
	mux.HandleFunc("/writestack/", s.handleWriteStack)
	mux.HandleFunc("/up/", s.handleProcessAction("up"))
	mux.HandleFunc("/down/", s.handleProcessAction("down"))

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
	useTLS := certFile != "" && keyFile != ""
	if useTLS {
		if _, err := os.Stat(certFile); err != nil {
			useTLS = false
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

func (s *ApiServer) getConn() *websocket.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn
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
		s.mu.Lock()
		s.conn = c
		s.mu.Unlock()
		log.Printf("✅ Connected to kdb hub at %s", url)
		s.readLoop(c)
	}
}

func (s *ApiServer) readLoop(conn *websocket.Conn) {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("❌ Kdb read error:", err)
			s.mu.Lock()
			s.conn = nil
			s.mu.Unlock()
			return
		}

		var resp KdbResponse
		if err := json.Unmarshal(message, &resp); err == nil {
			if resp.Callback == "upd" {
				s.broadcast(message)
				continue
			}
			if ch, ok := s.callbacks.Load(resp.Callback); ok {
				ch.(chan KdbResponse) <- resp
			}
		}
	}
}

func (s *ApiServer) kdbQuery(cmd string) (json.RawMessage, error) {
	conn := s.getConn()
	if conn == nil {
		return nil, fmt.Errorf("kdb hub not connected")
	}

	cbName := fmt.Sprintf("cb_%d", time.Now().UnixNano())
	ch := make(chan KdbResponse, 1)
	s.callbacks.Store(cbName, ch)
	defer s.callbacks.Delete(cbName)

	s.mu.Lock()
	err := conn.WriteJSON(KdbRequest{Callback: cbName, Cmd: cmd})
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case res := <-ch:
		if res.Error != "" {
			return nil, fmt.Errorf("%s", res.Error)
		}
		return res.Result, nil
	case <-timer.C:
		return nil, fmt.Errorf("kdb timeout")
	}
}

func writeJSON(w http.ResponseWriter, res json.RawMessage) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(res)
}

func (s *ApiServer) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `body must be {"cmd":"..."}`, http.StatusBadRequest)
		return
	}
	var req struct {
		Cmd string `json:"cmd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Cmd == "" {
		http.Error(w, `body must be {"cmd":"..."}`, http.StatusBadRequest)
		return
	}
	res, err := s.kdbQuery(req.Cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, res)
}

func (s *ApiServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	res, err := s.kdbQuery("0!procs")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, res)
}

func (s *ApiServer) handleListStacks(w http.ResponseWriter, r *http.Request) {
	res, err := s.kdbQuery("1_key .proc.stacks")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, res)
}

func (s *ApiServer) handleReadStack(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/readstack/")
	if name == "" {
		http.Error(w, "stack name required", http.StatusBadRequest)
		return
	}
	res, err := s.kdbQuery(fmt.Sprintf("raze readstack[`%s]", name))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// kdb+ returns the file contents as a JSON string — unwrap it
	var jsonStr string
	if err := json.Unmarshal(res, &jsonStr); err == nil {
		res = json.RawMessage(jsonStr)
	}
	writeJSON(w, res)
}

func (s *ApiServer) handleWriteStack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/writestack/")
	if name == "" {
		http.Error(w, "stack name required", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	if !json.Valid(body) {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	// Use json.Marshal to safely escape the body as a kdb+ string literal
	quoted, _ := json.Marshal(string(body))
	inner := string(quoted[1 : len(quoted)-1]) // strip surrounding quotes
	_, err = s.kdbQuery(fmt.Sprintf("writestack[`%s;enlist \"%s\"]", name, inner))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, json.RawMessage(fmt.Sprintf(`{"status":"ok","stack":"%s"}`, name)))
}

func (s *ApiServer) broadcast(msg []byte) {
	s.subscribers.Range(func(key, _ any) bool {
		ch := key.(chan []byte)
		select {
		case ch <- msg:
		default:
		}
		return true
	})
}

func (s *ApiServer) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan []byte, 16)
	s.subscribers.Store(ch, struct{}{})
	defer func() {
		s.subscribers.Delete(ch)
		close(ch)
	}()

	for {
		select {
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *ApiServer) handleProcessAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/"+action+"/")
		if name == "" {
			http.Error(w, "process name required", http.StatusBadRequest)
			return
		}
		_, err := s.kdbQuery(fmt.Sprintf("%s[`%s]", action, name))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, json.RawMessage(fmt.Sprintf(`{"status":"%s","process":"%s"}`, action, name)))
	}
}
