.PHONY: proto swagger

proto:
	protoc --go_out=. --go-grpc_out=. services/*/proto/*.proto

swagger:
	swag init -g main.go -d services/api-gateway -o services/api-gateway/docs
