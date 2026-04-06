.PHONY: proto proto-python swagger test

proto: proto-go proto-python

proto-go:
	@for dir in services/*/proto; do \
		service=$$(basename $$(dirname $$dir)); \
		if [ "$$service" = "ml-service" ]; then continue; fi; \
		echo "Generating proto for $$service..."; \
		protoc \
			--proto_path=$$dir \
			--go_out=$$dir --go_opt=paths=source_relative \
			--go-grpc_out=$$dir --go-grpc_opt=paths=source_relative \
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

test:
	@for dir in services/*/; do \
		if [ -f "$$dir/go.mod" ]; then \
			echo "Testing $$(basename $$dir)..."; \
			(cd $$dir && go test ./... -v) || exit 1; \
		fi; \
	done
