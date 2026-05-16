package strings

import (
	"math/rand"
	"time"
)
const charset = "abcdefghijklmnopqrstuvwxyz" +
  "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var randSeed *rand.Rand = rand.New(rand.NewSource(time.Now().Unix()))

func StringWithCharset(charset string, length int) string {

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[randSeed.Intn(len(charset))]
	}

	return string(b)

}


func RandomString(length int) string {
	return StringWithCharset(charset, length)
}
