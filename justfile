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
	go test -count=1 ./...

fmt:
	go fmt ./...

migrate-plan:
	cd migrations && atlas schema apply --env primary --dry-run

migrate:
	cd migrations && atlas schema apply --env primary

seed:
	go run ./cmd/christjesus seed

import-zips:
	go run ./cmd/christjesus import-zips

cognito-delete-user USERNAME:
	@if [ -z "${COGNITO_USER_POOL_ID}" ]; then echo "COGNITO_USER_POOL_ID is required"; exit 1; fi
	aws-vault exec cja -- aws cognito-idp admin-delete-user --user-pool-id "${COGNITO_USER_POOL_ID}" --username "{{USERNAME}}"

dev:
	air

e2e-reset:
	go run ./cmd/christjesus e2e-reset

e2e:
	just e2e-reset
	cd e2e && npx playwright test

e2e-headed:
	cd e2e && npx playwright test --headed

e2e-ui:
	cd e2e && npx playwright test --ui
