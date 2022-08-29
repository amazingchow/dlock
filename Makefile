lint:
	@golangci-lint run --deadline=5m

.PHONY: test_zk_dlock
test_zk_dlock:
	@echo "scale up zookeeper cluster"
	@docker-compose -f "test/docker-compose/docker-compose-zk-cluster.yml" up -d --build
	@sleep 3
	@go test -count 1 -v -p 1 dlock_by_zk_test.go dlock_by_zk.go client_zk_conn.go utils.go
	@echo "shutdown zookeeper cluster"
	@docker-compose -f "test/docker-compose/docker-compose-zk-cluster.yml" down

.PHONY: test_redis_dlock
test_redis_dlock:
	@echo "scale up zookeeper cluster"
	@docker-compose -f "test/docker-compose/docker-compose-redis-standalone.yml" up -d --build
	@sleep 3
	@go test -count 1 -v -p 1 dlock_by_redis_test.go dlock_by_redis.go client_redis_interface.go client_redis_conn_pool.go utils.go
	@echo "shutdown zookeeper cluster"
	@docker-compose -f "test/docker-compose/docker-compose-redis-standalone.yml" down
