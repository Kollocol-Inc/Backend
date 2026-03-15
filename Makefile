.PHONY: proto proto-python swagger

proto: proto-go proto-python

proto-go:
	@for dir in services/*/proto; do \
		service=$$(basename $$(dirname $$dir)); \
		if [ "$$service" = "ml-service" ]; then continue; fi; \
		echo "Generating proto for $$service..."; \
		protoc \
			--go_out=. --go_opt=paths=source_relative \
			--go-grpc_out=. --go-grpc_opt=paths=source_relative \
			$$dir/*.proto; \
	done

proto-python:
	python3 -m grpc_tools.protoc \
		--proto_path=services/ml-service/proto \
		--python_out=services/ml-service \
		--grpc_python_out=services/ml-service \
		services/ml-service/proto/ml.proto

swagger:
	swag init -g main.go -d services/api-gateway -o services/api-gateway/docs
