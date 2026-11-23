# 构建入口（设计文档 O.01 §4.1）。每个目标只有一行，调用 Go 工具或 pnpm。
# Windows 上不装 make 时，用 `task <目标>`（Taskfile.yaml）或直接执行下面的命令。
.PHONY: gen dev build test lint check e2e clean

gen:
	go run ./tools/gen

dev:
	go run ./tools/dev

build:
	pnpm -C web build && go run ./tools/build

test:
	go test ./... -count=1 && pnpm -C web test

lint:
	golangci-lint run ./... && go run ./tools/depcheck && go run ./tools/lint/quirkscheck && go run ./tools/lint/typenamecheck && pnpm -C web lint

check:
	go run ./tools/build -check

e2e:
	pnpm -C web build && go run ./tools/build && pnpm -C web test:e2e && pnpm -C web test:visual

clean:
	go run ./tools/build -clean
