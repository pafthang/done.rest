# Makefile to build and test the HiveOT Hub
DIST_FOLDER=./.space
BIN_FOLDER=./.space/bin
PLUGINS_FOLDER=./.space/mods
INSTALL_HOME=./.space/install
.DEFAULT_GOAL := help

.FORCE: 

all: core web cli   ## Build Core, Bindings and hubcli

# --- Core services

core: bus run cert dir hist prov state ## Build core services including mqttcore and natscore

# Build the embedded nats message bus core with auth
bus:
	go build -o $(BIN_FOLDER)/$@ done_mod/mod_bus/bus_cmd/main.go

run: .FORCE
	go build -o $(BIN_FOLDER)/$@ done_mod/mod_run/run_cmd/main.go
	mkdir -p $(DIST_FOLDER)/cfg
	cp done_cfg/*.yaml $(DIST_FOLDER)/cfg

cert: .FORCE
	go build -o $(PLUGINS_FOLDER)/$@ done_mod/mod_cert/cert_cmd/main.go

dir: .FORCE
	go build -o $(PLUGINS_FOLDER)/$@ done_mod/mod_dir/dir_cmd/main.go

hist: .FORCE
	go build -o $(PLUGINS_FOLDER)/$@ done_mod/mod_hist/hist_cmd/main.go

prov: .FORCE
	go build -o $(PLUGINS_FOLDER)/$@ done_mod/mod_prov/prov_cmd/main.go

state: .FORCE
	go build -o $(PLUGINS_FOLDER)/$@ done_mod/mod_state/state_cmd/main.go

# --- protocol bindings

web: .FORCE ## build the SSR web viewer binding
	go build -o $(PLUGINS_FOLDER)/$@ done_mod/mod_web/web_cmd/main.go

cli: .FORCE ## Build Done CLI
	go build -o $(BIN_FOLDER)/$@ done_cmd/cmd_done/main.go



clean: ## Clean distribution files
	go clean -cache -testcache -modcache
	rm -rf $(DIST_FOLDER)
	mkdir -p $(BIN_FOLDER)
	mkdir -p $(DIST_FOLDER)/mods
	mkdir -p $(DIST_FOLDER)/cert
	mkdir -p $(DIST_FOLDER)/cfg
	mkdir -p $(DIST_FOLDER)/logs
	mkdir -p $(DIST_FOLDER)/run
	go mod tidy
	go get all

help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'


install:  ## core plugins ## build and install the services
	mkdir -p $(INSTALL_HOME)/bin
	mkdir -p $(INSTALL_HOME)/mods
	mkdir -p $(INSTALL_HOME)/cert
	mkdir -p $(INSTALL_HOME)/cfg
	mkdir -p $(INSTALL_HOME)/logs
	mkdir -p $(INSTALL_HOME)/stores
	mkdir -p $(INSTALL_HOME)/run
	cp -af $(BIN_FOLDER)/* $(INSTALL_HOME)/bin
	cp -af $(PLUGINS_FOLDER)/* $(INSTALL_HOME)/mods
	cp -n $(DIST_FOLDER)/cfg/*.yaml $(INSTALL_HOME)/cfg/

test: core  ## Run tests (stop on first error, don't run parallel)
	go test -race -failfast -p 1 ./...

upgrade:
	go get -u all
	go mod tidy
