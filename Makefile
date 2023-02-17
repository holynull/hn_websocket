gen_protob: protob/*.proto
	protoc -I ./protob --go_out=. ./protob/*.proto 

run_ws:
	GOLOG_LOG_LEVEL="debug" go run main.go 