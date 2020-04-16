package pddlocks

import (
	"sync"
	"testing"
	"time"

	"github.com/samuel/go-zookeeper/zk"
	"github.com/stretchr/testify/assert"
)

var (
	fakeZKEndpoints = []string{"127.0.0.1:2181", "127.0.0.1:2182", "127.0.0.1:2183"}
)

func TestFastDLocker(t *testing.T) {
	conn, _ := EstablishZKConn(fakeZKEndpoints)
	defer CloseZKConn(conn)

	total := 0
	successCh := make(chan int, 20)
	go func() {
		for success := range successCh {
			total += success
		}
		assert.Equal(t, 20, total)
	}()

	var n sync.WaitGroup
	for i := 0; i < 20; i++ {
		n.Add(1)
		go func(conn *zk.Conn, idx int) {
			defer n.Done()

			dl := NewDLockByZookeeper(conn)
			if dl.TryLock(5) {
				time.Sleep(time.Microsecond * 100)
				dl.Unlock()
				successCh <- 1
			} else {
				successCh <- -1
			}
		}(conn, i)
	}
	n.Wait()

	close(successCh)
}
