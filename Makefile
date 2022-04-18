PACKAGES=$(shell go list ./...)
OUTPUT ?= /root/BFT/bft

CGO_ENABLED ?= 1

BUILD_FLAGS = -mod=readonly

VALIDATORS_NUM = 7

INDEX = 0

EVALUATION = false

del = ""

###############################################################################
###                          Build BFT State Machine                        ###
###############################################################################

build-bft:
	go build $(BUILD_FLAGS) -o $(OUTPUT) ./cmd/

install-bft:
	go install $(BUILD_FLAGS) ./cmd/
	cd $(GOPATH)/bin/ && mv cmd bft

install-bench:
	go install $(BUILD_FLAGS) ./benchmark/

benchmark:
	benchmark host localhost:36659
.PHONY: benchmark

build-docker-node:
	@cd benchmark/docker && make
.PHONY: build-docker-node

start-docker-net: stop-docker-net build-bft build-docker-node
	@if ! [ -f /root/BFT/node0/config/genesis.json ]; then docker run --rm -v /root/BFT:/bft:Z bft testnet --v $(VALIDATORS_NUM) --config /etc/bft/config-template.toml --o . --starting-ip-address 192.166.1.2; fi
	docker-compose up
.PHONY: start-docker-net

stop-docker-net:
	docker-compose down
.PHONY: stop-docker-net

clear:
	./clear.sh
.PHONY: clear