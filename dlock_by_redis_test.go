package dlock

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/petermattis/goid"
	"github.com/stretchr/testify/assert"
)

var (
	fakeRedisConnPoolConfig = &RedisConnPoolConfig{
		RedisEndpoint:       "127.0.0.1:6379",
		RedisDatabase:       0,
		RedisPassword:       "sOmE_sEcUrE_pAsS",
		RedisConnectTimeout: 2000,
		RedisReadTimeout:    1000,
		RedisWriteTimeout:   1000,
	}
)

func TestDlockByRedis(t *testing.T) {
	SkipAutoTest(t)

	rand.Seed(time.Now().UnixNano())

	conn := EstablishRedisConn(fakeRedisConnPoolConfig)
	defer conn.Close()

	total := 0

	wg := &sync.WaitGroup{}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(conn *RedisConnPool) {
			defer wg.Done()

			pid := fmt.Sprintf("%d", goid.Get())

			dl := NewDlockByRedis(conn)
			for {
				token, acquired := dl.TryLock(pid, 30000, 2)
				if acquired {
					defer dl.Unlock(pid, token)
					total++
					time.Sleep(time.Millisecond * time.Duration(10+rand.Intn(10)))
					break
				}
				time.Sleep(time.Millisecond * time.Duration(10+rand.Intn(10)))
			}
		}(conn)
	}
	wg.Wait()

	assert.Equal(t, 20, total)
}
