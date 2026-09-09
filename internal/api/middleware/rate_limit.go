package middleware

import (
	"container/list"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/mrbeaver1/dock-service/internal/api/response"
	"golang.org/x/time/rate"
)

const limiterCapacity = 10000
const limiterIdleTTL = 10 * time.Minute

type ipEntry struct {
	ip       string
	limiter  *rate.Limiter
	lastSeen time.Time
}

type IPLimiter struct {
	mu      sync.Mutex
	entries map[string]*list.Element
	recent  list.List
	rps     rate.Limit
	burst   int
}

func NewIPLimiter(rps rate.Limit, burst int) *IPLimiter {
	return &IPLimiter{entries: make(map[string]*list.Element), rps: rps, burst: burst}
}

func (il *IPLimiter) Allow(ip string) bool {
	il.mu.Lock()
	defer il.mu.Unlock()
	return il.allow(ip, time.Now())
}

func (il *IPLimiter) allow(ip string, now time.Time) bool {
	for first := il.recent.Front(); first != nil; first = il.recent.Front() {
		entry := first.Value.(*ipEntry)
		if now.Sub(entry.lastSeen) < limiterIdleTTL || entry.limiter.TokensAt(now) < float64(il.burst) {
			break
		}
		il.remove(first)
	}
	element := il.entries[ip]
	if element == nil {
		if len(il.entries) == limiterCapacity {
			il.remove(il.recent.Front())
		}
		element = il.recent.PushBack(&ipEntry{ip: ip, limiter: rate.NewLimiter(il.rps, il.burst)})
		il.entries[ip] = element
	}
	entry := element.Value.(*ipEntry)
	entry.lastSeen = now
	il.recent.MoveToBack(element)
	return entry.limiter.AllowN(now, 1)
}

func (il *IPLimiter) remove(element *list.Element) {
	delete(il.entries, element.Value.(*ipEntry).ip)
	il.recent.Remove(element)
}

func RateLimit(il *IPLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				// RemoteAddr без порта — берём как есть (тесты, unix-socket).
				ip = r.RemoteAddr
			}

			if !il.Allow(ip) {
				response.Fail(w, r, http.StatusTooManyRequests, http.StatusTooManyRequests, "too many requests")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
