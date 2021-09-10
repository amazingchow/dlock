lint:
	@golangci-lint run --deadline=5m

zk_test:
	@echo "scale up zookeeper cluster"
	docker-compose -f "test/docker-compose/docker-compose-zk-cluster.yml" up -d --build
	@sleep 3
	go test -count 1 -v -p 1 dlock_by_zk_test.go dlock_by_zk.go client_zk.go common.go
	@echo "shutdown zookeeper cluster"
	docker-compose -f "test/docker-compose/docker-compose-zk-cluster.yml" down

.PHONY: lint zk_test
