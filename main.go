package fiber_limiter

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/gofiber/fiber"
	"github.com/shareed2k/go_limiter"
)

const (
	SimpleAlgorithm  = "simple"
	GCRAAlgorithm    = "gcra"
	DefaultKeyPrefix = "fiber_limiter"
)

// Config ...
type Config struct {
	// Rediser
	Rediser *redis.Client

	// Max number of recent connections
	// Default: 10
	Max int

	// Burst
	Burst int

	// StatusCode
	// Default: 429 Too Many Requests
	StatusCode int

	// Message
	// default: "Too many requests, please try again later."
	Message string

	// Algorithm
	// Default: simple
	Algorithm string

	// Prefix
	// Default:
	Prefix string

	// Period
	Period time.Duration

	// Filter defines a function to skip middleware.
	// Optional. Default: nil
	Filter func(*fiber.Ctx) bool

	// Key allows to use a custom handler to create custom keys
	// Default: func(c *fiber.Ctx) string {
	//   return c.IP()
	// }
	Key func(*fiber.Ctx) string

	// Handler is called when a request hits the limit
	// Default: func(c *fiber.Ctx) {
	//   c.Status(cfg.StatusCode).SendString(cfg.Message)
	// }
	Handler func(*fiber.Ctx)
}

// New ...
func New(config Config) func(*fiber.Ctx) {
	if config.Rediser == nil {
		panic(errors.New("redis client is missing"))
	}

	if config.Handler == nil {
		config.Handler = func(c *fiber.Ctx) {
			c.Status(config.StatusCode).SendString(config.Message)
		}
	}

	if config.Key == nil {
		config.Key = func(c *fiber.Ctx) string {
			return c.IP()
		}
	}

	if config.Algorithm == "" {
		config.Algorithm = SimpleAlgorithm
	}

	if config.Period == 0 {
		config.Period = time.Minute
	}

	if config.Max == 0 {
		config.Max = 10
	}

	if config.Burst == 0 {
		config.Burst = 10
	}

	if config.Prefix == "" {
		config.Prefix = DefaultKeyPrefix
	}

	if config.Message == "" {
		config.Message = "Too many requests, please try again later."
	}

	if config.StatusCode == 0 {
		config.StatusCode = http.StatusTooManyRequests
	}

	limiter := go_limiter.NewLimiter(config.Rediser)
	limit := &go_limiter.Limit{
		Period:    config.Period,
		Algorithm: config.Algorithm,
		Rate:      int64(config.Max),
		Burst:     int64(config.Burst),
	}

	// override default limiter prefix
	limiter.Prefix = config.Prefix

	return func(c *fiber.Ctx) {
		// Filter request to skip middleware
		if config.Filter != nil && config.Filter(c) {
			c.Next()
			return
		}

		result, err := limiter.Allow(config.Key(c), limit)
		// if we have error lets just pass the request
		if err != nil {
			c.Next()
			return
		}

		// Check if hits exceed the max
		if !result.Allowed {
			// Call Handler func
			config.Handler(c)
			// Return response with Retry-After header
			// https://tools.ietf.org/html/rfc6584
			c.Set("Retry-After", strconv.FormatInt(time.Now().Add(result.RetryAfter).Unix(), 10))
			return
		}

		// We can continue, update RateLimit headers
		c.Set("X-RateLimit-Limit", strconv.Itoa(config.Max))
		c.Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
		c.Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(result.ResetAfter).Unix(), 10))
		// Bye!
		c.Next()
	}
}
