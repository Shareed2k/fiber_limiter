## fiber_limiter is middleware for fiber framework 

fiber_limiter using [redis](https://github.com/go-redis/redis) as store for rate limit with two algorithms for choosing simple, gcra [leaky bucket](https://en.wikipedia.org/wiki/Leaky_bucket)

### Install
```
go get -u github.com/gofiber/fiber
go get -u github.com/shareed2k/fiber_limiter
```
### Example
```go
package main

import (
	"log"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/gofiber/fiber"
	limiter "github.com/shareed2k/fiber_limiter"
)

func main() {
	app := fiber.New()

	option, err := redis.ParseURL("redis://127.0.0.1:6379/0")
	if err != nil {
		log.Fatal(err)
	}
	client := redis.NewClient(option)
	_ = client.FlushDB().Err()

	// 3 requests per 10 seconds max
	cfg := limiter.Config{
		Rediser:   client,
		Max:       3,
		Burst:     3,
		Period:    10 * time.Second,
		Algorithm: limiter.GCRAAlgorithm,
	}

	app.Use(limiter.New(cfg))

	app.Get("/", func(c *fiber.Ctx) {
		c.Send("Welcome!")
	})

	app.Listen(3000)
}

```
### Test
```curl
curl http://localhost:3000
...
< HTTP/1.1 200 OK
< Date: Fri, 03 Apr 2020 13:02:02 GMT
< Content-Type: text/plain; charset=utf-8
< Content-Length: 8
< X-Ratelimit-Limit: 3
< X-Ratelimit-Remaining: 2
< X-Ratelimit-Reset: 1585918925
...

curl http://localhost:3000
curl http://localhost:3000
curl http://localhost:3000

...
< HTTP/1.1 429 Too Many Requests
< Date: Fri, 03 Apr 2020 13:02:29 GMT
< Content-Type: text/plain; charset=utf-8
< Content-Length: 42
< Retry-After: 1585918951
...
```
