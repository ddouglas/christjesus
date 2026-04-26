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

webhooks:
  #!/usr/bin/env bash
  # Start ngrok tunnel to expose local webhook endpoints
  ngrok http 4318 > /dev/null 2>&1 &
  NGROK_PID=$!

  echo "Waiting for ngrok..."
  NGROK_URL=""
  for i in $(seq 1 15); do
    sleep 1
    NGROK_URL=$(curl -s http://localhost:4040/api/tunnels 2>/dev/null | jq -r '[.tunnels[] | select(.proto=="https")] | .[0].public_url' 2>/dev/null || true)
    if [ -n "$NGROK_URL" ] && [ "$NGROK_URL" != "null" ]; then
      break
    fi
  done

  if [ -z "$NGROK_URL" ] || [ "$NGROK_URL" = "null" ]; then
    echo "Error: could not get ngrok URL after 15 seconds"
    kill $NGROK_PID 2>/dev/null || true
    exit 1
  fi

  echo "ngrok tunnel: $NGROK_URL"

  resend webhooks listen --url "$NGROK_URL" --events all --forward-to localhost:8080/webhooks/resend &
  RESEND_PID=$!

  stripe listen --forward-to localhost:8080/webhooks/stripe &
  STRIPE_PID=$!

  trap "echo 'Shutting down webhooks...'; kill $NGROK_PID $RESEND_PID $STRIPE_PID 2>/dev/null || true" EXIT INT TERM

  wait

dev:
	#!/usr/bin/env bash
	export SOPS_AGE_KEY_FILE=~/.age/key.txt
	exec sops exec-env configs/local/app.enc.yaml 'air'

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
