# import env from files
-include .env

APP_NAME ?= price_monitor
APP_ENV ?= dev
APP_VERSION ?= dev

GOLANG_IMAGE ?= golang:latest
ALPINE_IMAGE ?= alpine:latest

BUILD_CMD ?= CGO_ENABLED=0 go build -tags=jsoniter -a -v -o bin/${APP_NAME} -ldflags '-v -w -s -linkmode auto -extldflags \"-static\" -X  main.AppName=${APP_NAME}  -X  main.AppVersion=${APP_VERSION}  -X  main.AppEnv=${APP_ENV}' ./cmd/${APP_NAME}

MACHINE_IP ?= 127.0.0.1
.EXPORT_ALL_VARIABLES:
DEV_POSTGRESQL_URL = 'postgres://${PM_POSTGRES_USER}:${PM_POSTGRES_PASSWORD}@localhost:5432/${PM_POSTGRES_USER}?sslmode=disable'

.PHONY: docker_clean_all
docker_clean_all:
	docker rm -f `docker ps -aq`
	docker volume rm -f `docker volume ls -q`
	#docker system prune

.PHONY: start
start:
	@echo "Run docker-compose"
	docker-compose -f docker-compose.yml up --scale pm-app=2

.PHONY: stop
stop:
	@echo "Stop docker-compose"
	docker-compose -f docker-compose.yml down

.PHONY: dev_env_up
dev_env_up:
	@echo "1. Run docker-compose dev"
	docker-compose -f .docker/docker-compose.override.yml up -d
	@echo "2. W8 ready"
	sleep 1
	./tools/check_postgres_ready.sh localhost 5432
	@echo "3. Running migrations"
	docker run --rm  -v ${PWD}/migrations:/migrations --network host migrate/migrate -path=/migrations/ -database ${DEV_POSTGRESQL_URL} up

.PHONY: dev_env_down
dev_env_down:
	@echo "Stop dev environment"
	docker-compose -f .docker/docker-compose.override.yml down

.PHONY: do_migrate
do_migrate:
	@echo "Running migration"
	docker run --rm -v ${PWD}/migrations/migrations:/migrations --network host migrate/migrate -path=/migrations/ -database ${POSTGRESQL_URL} up

.PHONY: tests
tests:
	@echo "Running go tests"
	go test -timeout 5m -race -short `go list ./... | grep -v postgres `  # except postgres dir

.PHONY: build
build:
	@echo "Running build"
	${BUILD_CMD}
