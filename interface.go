package pddlocks

// PhotonDanceDistributedLocks 分布式锁接口
// 底层可以通过zookeeper, etcd, redis实现.
type PhotonDanceDistributedLocks interface {
	TryLock(timeoutInSecs int) bool
	Unlock()
}
