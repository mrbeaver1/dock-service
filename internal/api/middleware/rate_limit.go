package middleware

import (
	"net"
	"net/http"

	"github.com/mrbeaver1/dock-service/internal/api/response"
	"golang.org/x/time/rate"
)

type IPLimiter struct {
	limiters typedSyncMap[string, *rate.Limiter]
	rps      rate.Limit
	burst    int
}

func NewIPLimiter(rps rate.Limit, burst int) *IPLimiter {
	return &IPLimiter{
		rps:   rps,
		burst: burst,
	}
}

func (il *IPLimiter) get(ip string) *rate.Limiter {
	if l, ok := il.limiters.Load(ip); ok {
		return l
	}

	l := rate.NewLimiter(il.rps, il.burst)
	actual, _ := il.limiters.LoadOrStore(ip, l)

	return actual
}

func RateLimit(il *IPLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				// RemoteAddr без порта — берём как есть (тесты, unix-socket).
				ip = r.RemoteAddr
			}

			if !il.get(ip).Allow() {
				response.Fail(w, r, http.StatusTooManyRequests, http.StatusTooManyRequests, "too many requests")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
