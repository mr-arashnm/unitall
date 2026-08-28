.PHONY: build test race vet fmt up down logs ps

build:
	go build ./...

test:
	go test ./... -count=1

race:
	go test ./... -race -count=1

vet:
	go vet ./...

fmt:
	gofmt -w .

# full stack (frontend + gateway + 6 services) from the repo-root compose
up:
	docker compose -f ../docker-compose.yml up --build -d

down:
	docker compose -f ../docker-compose.yml down

logs:
	docker compose -f ../docker-compose.yml logs -f --tail=50

ps:
	docker compose -f ../docker-compose.yml ps
