package pddlocks

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/go-zookeeper/zk"
	"github.com/petermattis/goid"
	"github.com/stretchr/testify/assert"
)

var (
	fakeZKEndpoints = []string{"127.0.0.1:2181", "127.0.0.1:2182", "127.0.0.1:2183"}
)

func TestDLockByZookeeper(t *testing.T) {
	SkipAutoTest(t)

	rand.Seed(time.Now().UnixNano())

	conn, _ := EstablishZKConn(fakeZKEndpoints, 0)
	defer CloseZKConn(conn)

	total := 0

	wg := &sync.WaitGroup{}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(conn *zk.Conn) {
			defer wg.Done()

			pid := fmt.Sprintf("%d", goid.Get())

			dl := NewDLockByZookeeper(conn)
			if dl.TryLock(pid, 2) {
				defer dl.Unlock(pid)
				total++
				time.Sleep(time.Millisecond * time.Duration(10+rand.Intn(10)))
			}
		}(conn)
	}
	wg.Wait()

	assert.Equal(t, 20, total)
}
