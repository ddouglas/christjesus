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

ngrok-resend:
  ngrok http 4318 &
  NGROK_PID=$!
  wait $NGROK_PID

resend-local:
  #!/usr/bin/env bash
  URL=$(curl -s http://localhost:4040/api/tunnels | jq -r '.tunnels[0].public_url')
  resend webhooks listen --url $URL --events all --forward-to localhost:8080/webhooks/resend

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

tf-init:
	cd terraform && terraform init

tf-plan workspace="development":
	#!/usr/bin/env bash
	set -euo pipefail
	TMPFILE=$(mktemp /tmp/terraform.tfvars.XXXXXX.json)
	trap "rm -f $TMPFILE" EXIT
	sops --decrypt configs/{{workspace}}/terraform.tfvars.enc.json > "$TMPFILE"
	cd terraform && terraform plan -var-file="$TMPFILE"

tf-apply workspace="development":
	#!/usr/bin/env bash
	set -euo pipefail
	TMPFILE=$(mktemp /tmp/terraform.tfvars.XXXXXX.json)
	trap "rm -f $TMPFILE" EXIT
	sops --decrypt configs/{{workspace}}/terraform.tfvars.enc.json > "$TMPFILE"
	cd terraform && terraform apply -var-file="$TMPFILE"

tf-console workspace="development":
	#!/usr/bin/env bash
	set -euo pipefail
	TMPFILE=$(mktemp /tmp/terraform.tfvars.XXXXXX.json)
	trap "rm -f $TMPFILE" EXIT
	sops --decrypt configs/{{workspace}}/terraform.tfvars.enc.json > "$TMPFILE"
	cd terraform && terraform console -var-file="$TMPFILE"

tf-state-list workspace="development":
	#!/usr/bin/env bash
	set -euo pipefail
	TMPFILE=$(mktemp /tmp/terraform.tfvars.XXXXXX.json)
	trap "rm -f $TMPFILE" EXIT
	sops --decrypt configs/{{workspace}}/terraform.tfvars.enc.json > "$TMPFILE"
	cd terraform && terraform state list
