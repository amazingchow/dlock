package dlock

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"time"

	"github.com/go-zookeeper/zk"
	log "github.com/sirupsen/logrus"
)

const (
	_DefaultZKConnSessionTimeout = time.Second * 10
	_DefaultZKConnMaxRetries     = 3

	_DlockRootPath                   = "/dlock"
	_DlockFastLockPath               = "/dlock/fast-lock"
	_DlockFastLockPathPrefix         = "/dlock/fast-lock/request-"
	_DlockFastLockPathShortestPrefix = "request-"
)

// EstablishZKConn 建立一条连接zookeeper集群的TCP连接.
func EstablishZKConn(endpoints []string, timeout int64 /* in secs */) (*zk.Conn, <-chan zk.Event) {
	rand.Seed(time.Now().UnixNano())

	sessionTimeout := _DefaultZKConnSessionTimeout
	if timeout > 0 {
		sessionTimeout = time.Second * time.Duration(timeout)
	}
	conn, evCh, err := zk.Connect(endpoints, sessionTimeout)
	if err != nil {
		log.WithError(err).Fatalf("failed to connect to zookeeper cluster (%v)", endpoints)
	}

WAIT_CONNECTED_LOOP:
	for {
		select {
		case connEv := <-evCh:
			if connEv.State == zk.StateConnected {
				break WAIT_CONNECTED_LOOP
			}
		case <-time.After(_DefaultZKConnSessionTimeout):
			conn.Close()
			log.WithError(err).Errorf("timeout to connect to zookeeper cluster (%v)", endpoints)
			return nil, nil
		}
	}

	zkCreateOrPanic(conn, _DlockRootPath)
	zkCreateOrPanic(conn, _DlockFastLockPath)
	return conn, evCh
}

// CloseZKConn 关闭TCP连接.
func CloseZKConn(conn *zk.Conn) {
	conn.Close()
}

// 如果ZNode不存在就创建.
func zkCreate(conn *zk.Conn, path string) error {
	if _, err := zkSafeCreateWithDefaultDataFilled(conn, path, 0); err != nil {
		return err
	}
	return nil
}

// 如果ZNode不存在就创建, 出错直接panic.
func zkCreateOrPanic(conn *zk.Conn, path string) {
	if err := zkCreate(conn, path); err != nil && err != zk.ErrNodeExists {
		log.WithError(err).Fatalf("failed to create znode <%s>", path)
	}
}

// 创建ZNode.
func zkSafeCreate(conn *zk.Conn, path string, data []byte, flags int32) (string, error) {
	var _path string
	var _data []byte
	var err error

	var retry bool
CREATE_LOOP:
	for i := 0; i < _DefaultZKConnMaxRetries; i++ {
		_path, err = conn.Create(path, data, flags, zk.WorldACL(zk.PermAll))
		switch err {
		case zk.ErrSessionExpired:
			{
				break CREATE_LOOP
			}
		// 连接关闭, 可能因为暂时的网络问题, 直接重试
		case zk.ErrConnectionClosed:
			{
				retry = true
				time.Sleep(time.Millisecond * time.Duration(10+rand.Intn(40)))
				continue
			}
		// ZNode已存在
		case zk.ErrNodeExists:
			{
				// 之前就创建过
				if !retry {
					err = zk.ErrNodeExists
					break CREATE_LOOP
				}
				// 因为网络问题导致的假失败
				_data, err = zkSafeGet(conn, path)
				if err != nil {
					// 又可能因为暂时的网络问题, 请重试
					time.Sleep(time.Millisecond * time.Duration(10+rand.Intn(40)))
					continue
				}
				if !bytes.Equal(data, _data) {
					err = zk.ErrUnknown
				}
				break CREATE_LOOP
			}
		default:
			{
				// TODO: 处理更多的错误情形
				break CREATE_LOOP
			}
		}
	}

	return _path, err
}

// 创建ZNode.
func zkSafeCreateWithDefaultDataFilled(conn *zk.Conn, path string, flags int32) (string, error) {
	var _path string
	var _data []byte
	var err error

	var retry bool

	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, uint64(time.Now().UnixMilli()))
CREATE_LOOP:
	for i := 0; i < _DefaultZKConnMaxRetries; i++ {
		_path, err = conn.Create(path, data, flags, zk.WorldACL(zk.PermAll))
		switch err {
		case zk.ErrSessionExpired:
			{
				break CREATE_LOOP
			}
		// 连接关闭, 可能因为暂时的网络问题, 直接重试
		case zk.ErrConnectionClosed:
			{
				retry = true
				time.Sleep(time.Millisecond * time.Duration(10+rand.Intn(40)))
				continue
			}
		// ZNode已存在
		case zk.ErrNodeExists:
			{
				// 之前就创建过
				if !retry {
					err = zk.ErrNodeExists
					break CREATE_LOOP
				}
				// 因为网络问题导致的假失败
				_data, err = zkSafeGet(conn, path)
				if err != nil {
					// 又可能因为暂时的网络问题, 请重试
					time.Sleep(time.Millisecond * time.Duration(10+rand.Intn(40)))
					continue
				}
				if !bytes.Equal(data, _data) {
					err = zk.ErrUnknown
				}
				break CREATE_LOOP
			}
		default:
			{
				// TODO: 处理更多的错误情形
				break CREATE_LOOP
			}
		}
	}

	return _path, err
}

// 获取ZNode的值.
func zkSafeGet(conn *zk.Conn, path string) ([]byte, error) {
	var _data []byte
	var err error

GET_LOOP:
	for i := 0; i < _DefaultZKConnMaxRetries; i++ {
		_data, _, err = conn.Get(path)
		switch err {
		case zk.ErrSessionExpired:
			{
				break GET_LOOP
			}
		// 连接关闭, 可能因为暂时的网络问题, 请重试
		case zk.ErrConnectionClosed:
			{
				time.Sleep(time.Millisecond * time.Duration(10+rand.Intn(40)))
				continue
			}
		default:
			{
				// TODO: 处理更多的错误情形
				break GET_LOOP
			}
		}
	}

	return _data, err
}

// 删除ZNode.
func zkSafeDelete(conn *zk.Conn, path string, version int32) error {
	var err error

	var retry bool
DELETE_LOOP:
	for i := 0; i < _DefaultZKConnMaxRetries; i++ {
		err = conn.Delete(path, version)
		switch err {
		case zk.ErrSessionExpired:
			{
				break DELETE_LOOP
			}
		// 连接关闭, 可能因为暂时的网络问题, 请重试
		case zk.ErrConnectionClosed:
			{
				retry = true
				time.Sleep(time.Millisecond * time.Duration(10+rand.Intn(40)))
				continue
			}
		// ZNode不存在
		case zk.ErrNoNode:
			{
				// 因为网络问题导致的假失败
				if !retry {
					err = zk.ErrNoNode
				}
				break DELETE_LOOP
			}
		default:
			{
				// TODO: 处理更多的错误情形
				break DELETE_LOOP
			}
		}
	}

	return err
}

// 获取ZNode下所有的子节点.
func zkSafeGetChildren(conn *zk.Conn, path string, watch bool) ([]string, <-chan zk.Event, error) {
	var _children []string
	var _watcher <-chan zk.Event
	var err error

GET_LOOP:
	for i := 0; i < _DefaultZKConnMaxRetries; i++ {
		if watch {
			_children, _, _watcher, err = conn.ChildrenW(path)
		} else {
			_children, _, err = conn.Children(path)
		}
		switch err {
		case zk.ErrSessionExpired:
			{
				break GET_LOOP
			}
		// 连接关闭, 可能因为暂时的网络问题, 请重试
		case zk.ErrConnectionClosed:
			{
				time.Sleep(time.Millisecond * time.Duration(10+rand.Intn(40)))
				continue
			}
		default:
			{
				// TODO: 处理更多的错误情形
				break GET_LOOP
			}
		}
	}

	return _children, _watcher, err
}
