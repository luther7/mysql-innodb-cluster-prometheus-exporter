
.PHONY: build
build:
	go build -o build/mysql_innodb_cluster_exporter

.PHONY: run
run: build
	./build/mysql_innodb_cluster_exporter

.PHONY: test
test:
	go test

.PHONY: docker.build
docker.build: build
	docker build --tag mysql-innodb-cluster-exporter .

.PHONY: up
up:
	docker-compose up --build --detach && docker-compose logs --follow

.PHONY: down
down:
	docker-compose down
