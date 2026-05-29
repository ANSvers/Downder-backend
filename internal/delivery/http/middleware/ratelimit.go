package middleware

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// IPRateLimiter โครงสร้างสำหรับเก็บสถานะ Rate Limit ของแต่ละ IP
type IPRateLimiter struct {
	ips map[string]*rate.Limiter
	mu  sync.RWMutex
	r   rate.Limit
	b   int
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{
		ips: make(map[string]*rate.Limiter),
		r:   r,
		b:   b,
	}
}

func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.ips[ip]
	if !exists {
		limiter = rate.NewLimiter(i.r, i.b)
		i.ips[ip] = limiter
	}

	return limiter
}

// LimitMiddleware ฟังก์ชันดักจับสำหรับการทำ Rate Limit
func LimitMiddleware(requestsPerMinute float64, burst int) gin.HandlerFunc {
	// คำนวณจำนวน Request ต่อวินาที (Rate limit แปลงเป็นวินาที)
	limitPerSecond := rate.Limit(requestsPerMinute / 60.0)
	limiter := NewIPRateLimiter(limitPerSecond, burst)

	return func(c *gin.Context) {
		ip := c.ClientIP()
		lim := limiter.GetLimiter(ip)

		if !lim.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please slow down.",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
