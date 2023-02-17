gen_protob: protob/*.proto
	protoc --go_out=. ./protob/*.proto 

run_ws:
	GOLOG_LOG_LEVEL="debug" go run main.go 