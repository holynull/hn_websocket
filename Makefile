gen_protob: protob/*.proto
	protoc -I ./protob --go_out=. ./protob/*.proto 

# test_1:
# 	go test -v -count=1 -timeout 300s -run ^TestDataMashall$ github.com/holynull/hn_websocket/mywebsocket

run_ws:
	GOLOG_LOG_LEVEL="debug" go run main.go 