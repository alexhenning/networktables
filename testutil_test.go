package networktables

import (
	"math/rand"
	"testing"
	"time"
)

func bytesToChannel(bytes []byte) <-chan byte {
	c := make(chan byte)
	go func() {
		for _, b := range bytes {
			c <- b
		}
		close(c)
	}()
	return c
}

func randomSource(t *testing.T) *rand.Rand {
	seed := time.Now().UnixNano()
	t.Logf("Seed: %d\n", seed)
	return rand.New(rand.NewSource(seed))
}

const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ.,!?/\\$&[{}(=*+)]!#`8642091357%~^@<>'\";:"

func randString(source *rand.Rand) string {
	length := uint16(source.Int31n(1 << 16))
	bytes := make([]byte, length, length)
	for i := uint16(0); i < length; i++ {
		bytes[i] = chars[source.Intn(len(chars))]
	}
	return string(bytes)
}
