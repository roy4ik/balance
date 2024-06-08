.PHONY: gen test-integration dev balance clean go-build-docker balance-build-docker balance-docker \
docker-clean-% docker-clean-containers-% docker-clean-images-% docker-clean-volumes-% docker-clean-networks-%

gen: 
    # make slb api 
	go generate ./...

test-integration:
	gen
	go test ./... -tags="integration"

# at this point only clean generated code and the final build dockers
clean: docker-clean-balance
	rm -rf ./gen

go-build-docker:
	echo "Building go build docker"
	docker build -f ./docker/Dockerfile.go-build -t go-build --cache-from go-build:latest .

balance-build-docker: go-build-docker
	echo "Building balance build docker"
	docker build -f ./docker/Dockerfile.balance-build -t balance-build --cache-from balance-build:latest .

balance-docker: balance-build-docker
	echo "Building balance"
	docker build -f ./docker/Dockerfile.balance -t balance --cache-from balance:latest .

# Default rule to clean all with specific prefix
docker-clean-%: docker-clean-containers-% docker-clean-images-% docker-clean-volumes-% docker-clean-networks-%
	@echo "Cleaning Docker $*"

# Clean Docker containers with the specified prefix
docker-clean-containers-%:
	@echo "Cleaning Docker containers with prefix: $*"
	@docker ps -a --filter "name=$*" --format "{{.ID}}" | xargs -r docker rm

# Clean Docker images with the specified prefix
docker-clean-images-%:
	@echo "Cleaning Docker images with prefix: $*"
	@docker images --format "{{.Repository}}:{{.Tag}}" | grep "^$*" | xargs -r docker rmi

# Clean Docker volumes with the specified prefix
docker-clean-volumes-%:
	@echo "Cleaning Docker volumes with prefix: $*"
	@docker volume ls --format "{{.Name}}" | grep "^$*" | xargs -r docker volume rm

# Clean Docker networks with the specified prefix
docker-clean-networks-%:
	@echo "Cleaning Docker networks with prefix: $*"
	@docker network ls --format "{{.Name}}" | grep "^$*" | xargs -r docker network rm

