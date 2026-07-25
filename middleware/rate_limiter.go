package middleware

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

var visitors = make(map[string]*rate.Limiter)
var mu sync.Mutex

// getVisitor mengambil limiter untuk IP tertentu, atau membuatnya jika belum ada
func getVisitor(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	limiter, exists := visitors[ip]
	if !exists {
		// Konfigurasi: 2 request per detik, maksimal burst 5 request
		// Sesuaikan angka ini dengan kebutuhan BurnCup
		limiter = rate.NewLimiter(3, 5)
		visitors[ip] = limiter
	}

	return limiter
}

// RateLimitMiddleware membatasi jumlah request per IP
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Mengambil IP asli dari client, aman meskipun di belakang proxy/Cloud Run
		ip := c.ClientIP()
		limiter := getVisitor(ip)

		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Terlalu banyak request. Silakan coba beberapa saat lagi.",
			})
			c.Abort() // Hentikan request agar tidak masuk ke handler
			return
		}

		c.Next()
	}
}
