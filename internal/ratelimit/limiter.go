package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"ripple-note/internal/middleware"
)

type Store interface {
	Increment(ctx context.Context, key string, window time.Duration) (int64, error)
}

type KeyFunc func(c *gin.Context) string

type Rule struct {
	Name    string
	Method  string
	Path    string
	Limit   int64
	Window  time.Duration
	KeyFunc KeyFunc
}

type Limiter struct {
	store Store
	rules []Rule
}

func NewLimiter(store Store, rules []Rule) *Limiter {
	filtered := make([]Rule, 0, len(rules))
	for _, rule := range rules {
		if rule.Method == "" || rule.Path == "" || rule.Limit <= 0 || rule.Window <= 0 || rule.KeyFunc == nil {
			continue
		}
		rule.Method = strings.ToUpper(rule.Method)
		filtered = append(filtered, rule)
	}
	return &Limiter{store: store, rules: filtered}
}

func (l *Limiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if l == nil || l.store == nil {
			c.Next()
			return
		}

		rule, ok := l.match(c)
		if !ok {
			c.Next()
			return
		}

		key := strings.TrimSpace(rule.KeyFunc(c))
		if key == "" {
			c.Next()
			return
		}

		count, err := l.store.Increment(c.Request.Context(), key, rule.Window)
		if err != nil {
			// Rate limiting protects the service but must not make business APIs
			// depend on Redis availability.
			c.Next()
			return
		}

		if count > rule.Limit {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"data": nil,
				"error": gin.H{
					"code":    "rate_limited",
					"message": "too many requests, please try again later",
				},
				"request_id": middleware.RequestIDFromContext(c),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (l *Limiter) match(c *gin.Context) (Rule, bool) {
	for _, rule := range l.rules {
		if rule.Method == c.Request.Method && rule.Path == c.FullPath() {
			return rule, true
		}
	}
	return Rule{}, false
}

func IPKey(prefix string) KeyFunc {
	return func(c *gin.Context) string {
		return fmt.Sprintf("%s:%s", strings.TrimRight(prefix, ":"), c.ClientIP())
	}
}

func UserIDKey(prefix string) KeyFunc {
	return func(c *gin.Context) string {
		userID, ok := userIDFromContext(c)
		if !ok || userID == 0 {
			return fmt.Sprintf("%s:anonymous:%s", strings.TrimRight(prefix, ":"), c.ClientIP())
		}
		return fmt.Sprintf("%s:%d", strings.TrimRight(prefix, ":"), userID)
	}
}

func userIDFromContext(c *gin.Context) (uint64, bool) {
	claims, ok := middleware.AuthClaimsFromContext(c)
	if !ok {
		return 0, false
	}
	return claims.UserID, true
}

type memoryEntry struct {
	count     int64
	expiresAt time.Time
}

type MemoryStore struct {
	mu    sync.Mutex
	items map[string]memoryEntry
	now   func() time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		items: map[string]memoryEntry{},
		now:   time.Now,
	}
}

func (s *MemoryStore) Increment(_ context.Context, key string, window time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	entry, ok := s.items[key]
	if !ok || !entry.expiresAt.After(now) {
		entry = memoryEntry{expiresAt: now.Add(window)}
	}
	entry.count++
	s.items[key] = entry
	return entry.count, nil
}
