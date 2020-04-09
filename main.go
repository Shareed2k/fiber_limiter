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
	SlidingWindowAlgorithm = go_limiter.SlidingWindowAlgorithm
	GCRAAlgorithm          = go_limiter.GCRAAlgorithm
	DefaultKeyPrefix       = "fiber_limiter"
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
	// Default: sliding window
	Algorithm uint

	// Prefix
	// Default:
	Prefix string

	// SkipOnError
	// Default: false
	SkipOnError bool

	// Period
	Period time.Duration

	// Filter defines a function to skip middleware.
	// Optional. Default: nil
	Filter func(*fiber.Ctx) bool

	// Key allows to use a custom handler to create custom keys
	// Default: func(ctx *fiber.Ctx) string {
	//   return ctx.IP()
	// }
	Key func(*fiber.Ctx) string

	// Handler is called when a request hits the limit
	// Default: func(ctx *fiber.Ctx) {
	//   ctx.Status(cfg.StatusCode).SendString(cfg.Message)
	// }
	Handler func(*fiber.Ctx)

	// ErrHandler is called when a error happen inside go_limiiter lib
	// Default: func(err error, ctx *fiber.Ctx) {
	//   ctx.Status(http.StatusInternalServerError).SendString(err.Error())
	// }
	ErrHandler func(error, *fiber.Ctx)
}

// New ...
func New(config Config) func(*fiber.Ctx) {
	if config.Rediser == nil {
		panic(errors.New("redis client is missing"))
	}

	if config.Handler == nil {
		config.Handler = func(ctx *fiber.Ctx) {
			ctx.Status(config.StatusCode).SendString(config.Message)
		}
	}

	if config.ErrHandler == nil {
		config.ErrHandler = func(err error, ctx *fiber.Ctx) {
			ctx.Status(http.StatusInternalServerError).SendString(err.Error())
		}
	}

	if config.Key == nil {
		config.Key = func(ctx *fiber.Ctx) string {
			return ctx.IP()
		}
	}

	if config.Algorithm == 0 {
		config.Algorithm = SlidingWindowAlgorithm
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

	return func(ctx *fiber.Ctx) {
		// Filter request to skip middleware
		if config.Filter != nil && config.Filter(ctx) {
			ctx.Next()

			return
		}

		result, err := limiter.Allow(config.Key(ctx), limit)
		// if we have error lets just pass the request
		if err != nil {
			if config.SkipOnError {
				ctx.Next()

				return
			}

			config.ErrHandler(err, ctx)

			return
		}

		// Check if hits exceed the max
		if !result.Allowed {
			// Call Handler func
			config.Handler(ctx)
			// Return response with Retry-After header
			// https://tools.ietf.org/html/rfc6584
			ctx.Set("Retry-After", strconv.FormatInt(time.Now().Add(result.RetryAfter).Unix(), 10))
			return
		}

		// We can continue, update RateLimit headers
		ctx.Set("X-RateLimit-Limit", strconv.Itoa(config.Max))
		ctx.Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
		ctx.Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(result.ResetAfter).Unix(), 10))
		// Bye!
		ctx.Next()
	}
}
