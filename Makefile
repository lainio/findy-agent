VERSION=$(shell cat ./VERSION)

API_BRANCH=$(shell ./branch.sh ../findy-agent-api/)
GRPC_BRANCH=$(shell ./branch.sh ../findy-common-go/)
WRAP_BRANCH=$(shell ./branch.sh ../findy-wrapper-go/)

drop_wrap:
	go mod edit -dropreplace github.com/findy-network/findy-wrapper-go

drop_comm:
	go mod edit -dropreplace github.com/findy-network/findy-common-go

drop_api:
	go mod edit -dropreplace github.com/findy-network/findy-agent-api

drop_all: drop_api drop_comm drop_wrap

repl_wrap:
	go mod edit -replace github.com/findy-network/findy-wrapper-go=../fingy-wrapper-go

repl_comm:
	go mod edit -replace github.com/findy-network/findy-common-go=../fingy-common-go

repl_api:
	go mod edit -replace github.com/findy-network/findy-agent-api=../fingy-agent-api

repl_all: repl_api repl_comm repl_wrap

modules:
	@echo Syncing modules for work brances ...
	go get github.com/findy-network/findy-agent-api@$(API_BRANCH)
	go get github.com/findy-network/findy-common-go@$(GRPC_BRANCH)
	go get github.com/findy-network/findy-wrapper-go@$(WRAP_BRANCH)

deps:
	go get -t ./...

update-deps:
	go get -u ./...

build:
	go build -v ./...

vet:
	go vet ./...

install:
	@echo "Installing version $(VERSION)"
	go install \
		-ldflags "-X 'github.com/findy-network/findy-agent/agent/utils.Version=$(VERSION)'" \
		./...

shadow:
	@echo Running govet
	go vet -vettool=$(GOPATH)/bin/shadow ./...
	@echo Govet success

check_fmt:
	$(eval GOFILES = $(shell find . -name '*.go'))
	@gofmt -s -l $(GOFILES)

#lint:
#	$(GOPATH)/bin/golint ./... 

lint_e:
	@$(GOPATH)/bin/golint ./... | grep -v export | cat

lint:
	@golangci-lint run

test:
	go test -p 1 -failfast ./...

testv:
	go test -v -p 1 -failfast ./...

test_cov:
	go test -v -p 1 -failfast -coverprofile=c.out ./... && go tool cover -html=c.out

check: check_fmt vet shadow

