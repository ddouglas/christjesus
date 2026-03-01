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

cognito-delete-user USERNAME:
	@if [ -z "${COGNITO_USER_POOL_ID}" ]; then echo "COGNITO_USER_POOL_ID is required"; exit 1; fi
	aws-vault exec cja -- aws cognito-idp admin-delete-user --user-pool-id "${COGNITO_USER_POOL_ID}" --username "{{USERNAME}}"

dev:
	air 
