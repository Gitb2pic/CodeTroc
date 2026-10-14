package httpapi

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateRule autorise Burst requêtes d'affilée, puis Burst par période Per.
type RateRule struct {
	Burst int
	Per   time.Duration
}

// RateLimits règle la limitation de débit (en mémoire, par instance d'API).
type RateLimits struct {
	Auth  RateRule // /auth/* par adresse IP : connexion, rafraîchissement
	IP    RateRule // toute l'API par adresse IP (hors webhooks, signés)
	Write RateRule // écritures par membre connecté
}

// DefaultRateLimits convient à un usage humain normal et à l'interrogation
// périodique du front, et coupe court aux rafales automatisées.
func DefaultRateLimits() RateLimits {
	return RateLimits{
		Auth:  RateRule{Burst: 60, Per: time.Minute}, // chaque chargement de page rafraîchit la session ; NAT partagés
		IP:    RateRule{Burst: 600, Per: time.Minute},
		Write: RateRule{Burst: 60, Per: time.Minute},
	}
}

// ErrTooManyRequests est renvoyé avec Retry-After quand un seau est vide.
var ErrTooManyRequests = Problem{Type: "/problems/trop-de-requetes", Title: "Trop de requêtes", Status: http.StatusTooManyRequests,
	Detail: "Ralentis un peu, puis réessaie."}

// maxTrackedKeys borne la mémoire du limiteur face à une rotation d'adresses.
const maxTrackedKeys = 100_000

type bucket struct {
	tokens float64
	at     time.Time
}

// limiter est un seau à jetons par clé.
type limiter struct {
	mu      sync.Mutex
	rule    RateRule
	perTok  time.Duration
	now     func() time.Time
	buckets map[string]*bucket
}

func newLimiter(rule RateRule, now func() time.Time) *limiter {
	return &limiter{rule: rule, perTok: rule.Per / time.Duration(rule.Burst), now: now, buckets: map[string]*bucket{}}
}

func (l *limiter) refill(b *bucket, now time.Time) {
	b.tokens = math.Min(float64(l.rule.Burst), b.tokens+float64(now.Sub(b.at))/float64(l.perTok))
	b.at = now
}

// allow consomme un jeton ; sinon indique quand le prochain sera disponible.
func (l *limiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= maxTrackedKeys {
			l.forgetFull(now)
		}
		b = &bucket{tokens: float64(l.rule.Burst), at: now}
		l.buckets[key] = b
	}
	l.refill(b, now)
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	return false, time.Duration((1 - b.tokens) * float64(l.perTok)).Round(time.Second)
}

// forgetFull oublie les seaux revenus pleins : ils équivalent à une clé inconnue.
func (l *limiter) forgetFull(now time.Time) {
	for k, b := range l.buckets {
		l.refill(b, now)
		if b.tokens >= float64(l.rule.Burst) {
			delete(l.buckets, k)
		}
	}
}

// rateLimit applique le limiteur à la clé calculée ; une clé vide passe.
func rateLimit(l *limiter, key func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		k := key(c)
		if k == "" {
			c.Next()
			return
		}
		if ok, retry := l.allow(k); !ok {
			c.Header("Retry-After", strconv.Itoa(int(math.Max(1, retry.Seconds()))))
			WriteProblem(c, ErrTooManyRequests)
			return
		}
		c.Next()
	}
}
