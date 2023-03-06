package dlock

import (
	"crypto/rc4"
	"encoding/hex"
	"math/rand"
	"time"

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
	rdb    RedisConnInterface
	cipher *rc4.Cipher
}

// NewDlockByRedis 获取DlockByRedis实例.
func NewDlockByRedis(rdb RedisConnInterface) *DlockByRedis {
	inst := &DlockByRedis{
		rdb: rdb,
	}
	key := make([]byte, 32)
	rand.Read(key)
	inst.cipher, _ = rc4.NewCipher(key)
	return inst
}

// TryLock 尝试获取分布式锁, 超时后就放弃 (不可重入锁).
func (dlr *DlockByRedis) TryLock(pid string, expire /* in milliseconds  */, timeout int64 /* in seconds */) (token string, acquired bool) {
	if expire <= 0 {
		expire = 30000
	}
	if timeout <= 0 {
		timeout = 1
	}

	rv := dlr.random()
	timeout = time.Now().Unix() + timeout

LOOP:
	for {
		v, err := dlr.rdb.ExecCmd("SET", _DlockRedisKey, rv, "NX", "PX", expire)
		if err != nil {
			log.WithField("pid", pid).WithError(err).Error("failed to acquire lock")
			acquired = false
			break LOOP
		}
		if v != nil && v.(string) == "OK" {
			token = rv
			acquired = true
			break LOOP
		}
		if time.Now().Unix() > timeout {
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
		log.WithField("pid", pid).WithError(err).Error("failed to release lock")
	}
	if v == nil || v.(int64) != 1 {
		log.WithField("pid", pid).WithError(err).Error("failed to release lock")
	}
}

func (dlr *DlockByRedis) random() string {
	src := make([]byte, 20)
	rand.Read(src)
	dst := make([]byte, 20)
	dlr.cipher.XORKeyStream(dst, src)
	return hex.EncodeToString(dst)
}
