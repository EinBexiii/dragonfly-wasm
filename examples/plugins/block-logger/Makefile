.PHONY: build clean install check fmt

PLUGIN_NAME := block_logger_plugin
TARGET := wasm32-unknown-unknown
OUT_DIR := ../../../plugins/block-logger

build:
	cargo build --release --target $(TARGET)
	@mkdir -p $(OUT_DIR)
	cp target/$(TARGET)/release/$(PLUGIN_NAME).wasm $(OUT_DIR)/plugin.wasm
	cp plugin.toml $(OUT_DIR)/

clean:
	cargo clean
	rm -rf $(OUT_DIR)

install: build

check:
	cargo check --target $(TARGET)

fmt:
	cargo fmt

lint:
	cargo clippy --target $(TARGET) -- -D warnings
