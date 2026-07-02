.PHONY: build test lint mock deploy synth

build:
	$(MAKE) -C backend build
	cd infra && go build ./...

test:
	$(MAKE) -C backend test

lint:
	$(MAKE) -C backend lint

mock:
	$(MAKE) -C backend mock

synth:
	cd infra && cdk synth

deploy:
	cd infra && cdk deploy
