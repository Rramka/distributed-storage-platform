package lifecycle

import (
	"testing"
	"time"
)

func TestTickPositive(t *testing.T) {
	t.Parallel()
	if tick < time.Second {
		t.Fatal(tick)
	}
}
