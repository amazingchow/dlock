package dlock

import (
	"context"
	"crypto/tls"
	"errors"
	"time"

	"github.com/FZambia/sentinel"
	"github.com/gomodule/redigo/redis"
	log "github.com/sirupsen/logrus"
)

type RedisHAConnPool struct {
	db int
	p  *redis.Pool
}

type RedisHAConnPoolConfig struct {
	SentinelEndpoints       []string `json:"sentinel_endpoints"`
	SentinelMasterName      string   `json:"sentinel_master_name"`
	SentinelPassword        string   `json:"sentinel_password"`
	SentinelOpenTLS         bool     `json:"sentinel_open_tls"`
	RedisDatabase           int      `json:"redis_database"`
	RedisMasterPassword     string   `json:"redis_master_password"`
	RedisConnectTimeout     int      `json:"redis_connect_timeout_msec"`  // 连接超时
	RedisReadTimeout        int      `json:"redis_read_timeout_msec"`     // 读取超时
	RedisWriteTimeout       int      `json:"redis_write_timeout_msec"`    // 写入超时
	RedisPoolMaxIdleConns   int      `json:"redis_pool_max_idle_conns"`   // 连接池最大空闲连接数
	RedisPoolMaxActiveConns int      `json:"redis_pool_max_active_conns"` // 连接池最大激活连接数
}

// EstablishRedisHAConnPool 建立连接redis服务的TCP连接池.
func EstablishRedisHAConnPool(cfg *RedisHAConnPoolConfig) *RedisHAConnPool {
	sntnl := &sentinel.Sentinel{
		Addrs:      cfg.SentinelEndpoints,
		MasterName: cfg.SentinelMasterName,
		Dial: func(addr string) (redis.Conn, error) {
			opts := make([]redis.DialOption, 0)
			opts = append(opts, redis.DialPassword(cfg.SentinelPassword))
			if cfg.RedisConnectTimeout > 0 {
				opts = append(opts, redis.DialConnectTimeout(time.Duration(cfg.RedisConnectTimeout)*time.Millisecond))
			}
			if cfg.SentinelOpenTLS {
				opts = append(opts, redis.DialUseTLS(true))
				opts = append(opts, redis.DialTLSConfig(&tls.Config{}))
				opts = append(opts, redis.DialTLSSkipVerify(true))
			} else {
				opts = append(opts, redis.DialUseTLS(false))
			}

			conn, err := redis.DialContext(context.Background(), "tcp", addr, opts...)
			if err != nil {
				log.WithError(err).Fatal("failed to connect to redis server")
				return nil, err
			}
			return conn, nil
		},
	}

	instance := &RedisHAConnPool{}
	instance.db = cfg.RedisDatabase
	instance.p = &redis.Pool{
		Dial: func() (redis.Conn, error) {
			addr, err := sntnl.MasterAddr()
			if err != nil {
				log.WithError(err).Fatal("failed to connect to redis server")
				return nil, err
			}

			opts := make([]redis.DialOption, 0)
			opts = append(opts, redis.DialDatabase(cfg.RedisDatabase))
			opts = append(opts, redis.DialPassword(cfg.RedisMasterPassword))
			if cfg.RedisReadTimeout > 0 {
				opts = append(opts, redis.DialReadTimeout(time.Duration(cfg.RedisReadTimeout)*time.Millisecond))
			}
			if cfg.RedisWriteTimeout > 0 {
				opts = append(opts, redis.DialWriteTimeout(time.Duration(cfg.RedisWriteTimeout)*time.Millisecond))
			}

			conn, err := redis.DialContext(context.Background(), "tcp", addr, opts...)
			if err != nil {
				log.WithError(err).Fatal("failed to connect to redis server")
				return nil, err
			}
			return conn, nil
		},
		TestOnBorrow: func(conn redis.Conn, t time.Time) error {
			if !sentinel.TestRole(conn, "master") {
				return errors.New("role check failed")
			} else {
				return nil
			}
		},
		MaxIdle:   cfg.RedisPoolMaxIdleConns,
		MaxActive: cfg.RedisPoolMaxActiveConns,
		Wait:      true,
	}
	return instance
}

// CloseRedisConn 释放TCP连接池.
func (p *RedisHAConnPool) Close() {
	if p != nil {
		_ = p.p.Close()
	}
}

// ExecCommand 执行redis命令, 完成后自动归还连接.
func (p *RedisHAConnPool) ExecCmd(cmd string, args ...interface{}) (interface{}, error) {
	conn, err := p.getConn()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return conn.Do(cmd, args...)
}

// ExecLuaScript 执行lua脚本, 完成后自动归还连接.
func (p *RedisHAConnPool) ExecLuaScript(src string, keyCount int, keysAndArgs ...interface{}) (interface{}, error) {
	conn, err := p.getConn()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	luaScript := redis.NewScript(keyCount, src)
	return luaScript.Do(conn, keysAndArgs...)
}

func (p *RedisHAConnPool) getConn() (conn redis.Conn, err error) {
	conn = p.p.Get()
	_, err = conn.Do("SELECT", p.db)
	return
}
