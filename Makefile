# Config
PROTO_DIR := proto
GO_OUT_DIR := pkg/logproto
DESC_OUT := schema.desc

PROTO_FILES := $(shell find $(PROTO_DIR) -name '*.proto')

# Targets

.PHONY: all proto clean

all: proto

# Generate Go code + descriptor set
proto: $(PROTO_FILES)
	@echo "Generating Go protobufs..."
	@mkdir -p $(GO_OUT_DIR)
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(GO_OUT_DIR) \
		--go_opt=paths=source_relative \
		--descriptor_set_out=$(DESC_OUT) \
		--include_imports \
		$^

	@echo "Proto generation complete."
	@echo " - Go output: $(GO_OUT_DIR)"
	@echo " - Descriptor set: $(DESC_OUT)"

clean:
	@echo "Cleaning generated files..."
	rm -f $(DESC_OUT)
	find $(GO_OUT_DIR) -name '*.pb.go' -delete
	@echo "Clean done."
