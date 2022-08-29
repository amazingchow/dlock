package dlock

import (
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const (
	_DlockRedisKey = "dlock"

	// -1: failed to get; 0: failed to del;  1: success to del
	_CheckAndDel = `if redis.call('get', KEYS[1]) == ARGV[1] then
	return redis.call('del', KEYS[1])
else
	return -1
end`
)

// DlockByRedis 通过redis实现的分布式锁服务
type DlockByRedis struct {
	rdb RedisConnInterface
}

// NewDlockByRedis 获取DlockByRedis实例.
func NewDlockByRedis(rdb RedisConnInterface) *DlockByRedis {
	return &DlockByRedis{
		rdb: rdb,
	}
}

// TryLock 尝试获取分布式锁, 超时后就放弃 (不可重入锁).
func (dlr *DlockByRedis) TryLock(pid string, timeout int64 /* in secs */) (token string, acquired bool) {
	if timeout <= 0 {
		timeout = 1
	}

	uid := uuid.New().String()
	ttl := time.Now().Unix() + timeout

LOOP:
	for {
		// TODO: 为了避免出现死锁状态, 需要设置一个合理的过期时间, 设置为多少比较合理?
		v, err := dlr.rdb.ExecCmd("SET", _DlockRedisKey, uid, "NX", "EX", 5)
		if err != nil {
			log.WithField("pid", pid).WithError(err).Error("failed to acquire lock")
			acquired = false
			break LOOP
		}
		if v == nil {
			continue
		}
		if v.(string) == "OK" {
			token = uid
			acquired = true
			break LOOP
		}
		if time.Now().Unix() > ttl {
			log.WithField("pid", pid).Warn("timeout to acquire lock")
			acquired = false
			break LOOP
		}
	}

	return
}

// Unlock 释放分布式锁.
func (dlr *DlockByRedis) Unlock(pid, token string) {
	v, err := dlr.rdb.ExecLuaScript(_CheckAndDel, 1, _DlockRedisKey, token)
	if err != nil {
		log.WithError(err).Error("failed to release lock")
	}
	if v == nil || v.(int64) != 1 {
		log.WithError(err).Error("failed to release lock")
	}
}
