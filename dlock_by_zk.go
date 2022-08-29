package pddlocks

import (
	"strconv"
	"strings"
	"time"

	"github.com/go-zookeeper/zk"
	log "github.com/sirupsen/logrus"
)

// DLockByZookeeper 通过zookeeper实现的分布式锁服务
type DLockByZookeeper struct {
	conn  *zk.Conn
	lpath string
}

// NewDLockByZookeeper 获取DLockByZookeeper实例.
func NewDLockByZookeeper(conn *zk.Conn) *DLockByZookeeper {
	return &DLockByZookeeper{
		conn: conn,
	}
}

// TryLock 尝试获取分布式锁, 超时后就放弃 (不可重入锁).
/*

==> acquire lock
n = create("/dlock/fast-lock/request-", "", ephemeral|sequence)
RETRY:
    children = getChildren("/dlock/fast-lock", watch=False)
    if n is lowest znode in children:
        return
    else:
        exist("/dlock/fast-lock/request-" % (n - 1), watch=True)

watch_event:
	goto RETRY

*/
func (dlz *DLockByZookeeper) TryLock(pid string, timeout int64 /* in secs */) bool {
	path, err := zkSafeCreateWithDefaultDataFilled(dlz.conn, _DlockFastLockPathPrefix, zk.FlagEphemeral|zk.FlagSequence)
	if err != nil {
		log.WithField("pid", pid).WithError(err).Error("failed to acquire lock")
		return false
	}
	seq := dlz.getSequenceNum(path, _DlockFastLockPathPrefix)

	ticker1 := time.NewTicker(time.Duration(timeout) * time.Second)
	defer ticker1.Stop()
	ticker2 := time.NewTicker(time.Duration(200) * time.Millisecond)
	defer ticker2.Stop()
LOOP:
	for {
		select {
		case <-ticker1.C:
			{
				log.WithField("pid", pid).Warn("timeout to acquire lock")
				return false
			}
		default:
			{
			TRY_AGAIN:
				children, _, err := zkSafeGetChildren(dlz.conn, _DlockFastLockPath, false)
				if err != nil {
					log.WithField("pid", pid).WithError(err).Error("failed to acquire lock")
					return false
				}

				minSeq := seq
				prevSeq := -1
				prevSeqPath := ""
				for _, child := range children {
					_seq := dlz.getSequenceNum(child, _DlockFastLockPathShortestPrefix)
					if _seq < minSeq {
						minSeq = _seq
					}
					if _seq < seq && _seq > prevSeq {
						prevSeq = _seq
						prevSeqPath = child
					}
				}
				if seq == minSeq {
					break LOOP
				}

				_, _, watcher, err := dlz.conn.ExistsW(_DlockFastLockPath + "/" + prevSeqPath)
				if err != nil {
					log.WithField("pid", pid).WithError(err).Error("failed to acquire lock")
					return false
				}

				ticker2.Reset(time.Duration(200) * time.Millisecond)
				for {
					select {
					case ev, ok := <-watcher:
						{
							if !ok {
								return false
							}
							if ev.Type == zk.EventNodeDeleted {
								goto TRY_AGAIN
							}
						}
					case <-ticker2.C:
						{
							goto TRY_AGAIN
						}
					}
				}
			}
		}
	}

	dlz.lpath = path
	return true
}

// Unlock 释放分布式锁.
/*

==> release lock (voluntarily or session timeout)
delete("/dlock/fast-lock/request-" % n)

*/
func (dlz *DLockByZookeeper) Unlock(pid string) {
	if err := zkSafeDelete(dlz.conn, dlz.lpath, -1); err != nil {
		log.WithField("pid", pid).WithError(err).Error("failed to release lock")
	}
	dlz.lpath = ""
}

func (dlz *DLockByZookeeper) getSequenceNum(path, prefix string) int {
	numStr := strings.TrimPrefix(path, prefix)
	num, _ := strconv.Atoi(numStr)
	return num
}
