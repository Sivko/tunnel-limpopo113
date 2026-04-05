package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"sync"
)

func main() {
	listen := envOr("LISTEN_ADDR", ":8080")
	path := envOr("WHITELIST_FILE", "/data/whitelist.json")
	token := strings.TrimSpace(os.Getenv("WHITELIST_ADMIN_TOKEN"))
	if token == "" {
		log.Fatal("WHITELIST_ADMIN_TOKEN is required")
	}

	store, err := newStore(path)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /white-list", func(w http.ResponseWriter, r *http.Request) {
		if !bearerOK(r, token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(store.list())
	})
	mux.HandleFunc("POST /white-list", func(w http.ResponseWriter, r *http.Request) {
		if !bearerOK(r, token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var raw []string
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		normalized, err := normalizeEntries(raw)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := store.replace(normalized); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(normalized)
	})
	mux.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		ip, ok := clientIP(r)
		if !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if !store.allowed(ip) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	log.Printf("listening on %s", listen)
	if err := http.ListenAndServe(listen, mux); err != nil {
		log.Fatal(err)
	}
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func bearerOK(r *http.Request, want string) bool {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return false
	}
	got := strings.TrimSpace(h[len(prefix):])
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func clientIP(r *http.Request) (netip.Addr, bool) {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			if a, err := netip.ParseAddr(strings.TrimSpace(parts[0])); err == nil {
				return a, true
			}
		}
	}
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		if a, err := netip.ParseAddr(strings.TrimSpace(xri)); err == nil {
			return a, true
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		if a, e := netip.ParseAddr(r.RemoteAddr); e == nil {
			return a, true
		}
		return netip.Addr{}, false
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return a, true
}

type store struct {
	mu       sync.RWMutex
	path     string
	entries  []string
	prefixes []netip.Prefix
}

func newStore(path string) (*store, error) {
	s := &store{path: path}
	if b, err := os.ReadFile(path); err == nil {
		var entries []string
		if err := json.Unmarshal(b, &entries); err != nil {
			return nil, err
		}
		normalized, err := normalizeEntries(entries)
		if err != nil {
			return nil, err
		}
		ps, err := parseEntries(normalized)
		if err != nil {
			return nil, err
		}
		s.entries = append([]string(nil), normalized...)
		s.prefixes = ps
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return s, nil
}

func normalizeEntries(entries []string) ([]string, error) {
	var out []string
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if _, err := parseEntry(e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func parseEntries(entries []string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		p, err := parseEntry(e)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func parseEntry(s string) (netip.Prefix, error) {
	if strings.Contains(s, "/") {
		return netip.ParsePrefix(s)
	}
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Prefix{}, err
	}
	if a.Is4() {
		return a.Prefix(32)
	}
	return a.Prefix(128)
}

func (s *store) list() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.entries))
	copy(out, s.entries)
	return out
}

func (s *store) replace(normalized []string) error {
	ps, err := parseEntries(normalized)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append([]string(nil), normalized...)
	s.prefixes = ps
	data, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

func (s *store) allowed(ip netip.Addr) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.prefixes) == 0 {
		return false
	}
	for _, p := range s.prefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}
