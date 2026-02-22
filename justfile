set dotenv-load

default:
	@just --list

deps:
	go mod tidy

serve:
	go run ./cmd/christjesus serve

build:
	go build -o ./.bin/christjesus ./cmd/christjesus

test:
	go test ./...

fmt:
	go fmt ./...

migrate-plan:
	cd migrations && atlas schema apply --env primary --dry-run

migrate:
	cd migrations && atlas schema apply --env primary --auto-approve

seed:
	go run ./cmd/christjesus seed

dev:
	air 