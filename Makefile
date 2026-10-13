.PHONY: test web run

test:
	go test ./...

web:
	cd web && npm install && npm run build

run: web
	go run ./cmd/server
