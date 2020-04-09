package fiber_limiter

import (
	"testing"

	"github.com/gofiber/fiber"
	"github.com/stretchr/testify/assert"
)

func TestPanicOnNilRediser(t *testing.T) {
	ff := func() {
		New(Config{})(&fiber.Ctx{})
	}

	assert.Panics(t, ff, "should panic on rediser nil")
}

func TestSkipOnError(t *testing.T) {
	app := fiber.New()

}
