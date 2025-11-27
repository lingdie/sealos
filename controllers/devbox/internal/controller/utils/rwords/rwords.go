package rwords

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

const listLength = len(wordsList)

var (
	rnd  *rand.Rand
	once sync.Once
)

func getRand() *rand.Rand {
	once.Do(func() {
		rnd = rand.New(rand.NewSource(time.Now().UnixNano()))
	})
	return rnd
}

// data size is 2048*2048*100000 = 419,430,400,000
// compare to 8 random characters and 10 digits, data size is 36^8 = 2,821,109,907,456
// outputs looks like: "abandon-ability-12345"
func GenerateRandomWords() string {
	r := getRand()
	return fmt.Sprintf("%s-%s-%d", wordsList[r.Intn(listLength)], wordsList[r.Intn(listLength)], r.Intn(100000))
}
