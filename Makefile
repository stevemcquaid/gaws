## Build container
build:
	@bash -x scripts/build.sh

## Push container
push:
	@bash -x scripts/push.sh

## Run example creation of EC2 Instance
run:
	@bash -x scripts/build.sh
	@bash -x scripts/run.sh

## Run a shell in container locally for debugging
shell:
	@bash -x scripts/shell.sh


help:
	@printf "Available make targets:\n\n"
	@awk '/^[a-zA-Z\-\_0-9%:\\]+:/ { \
	  helpMessage = match(lastLine, /^## (.*)/); \
	  if (helpMessage) { \
	    helpCommand = $$1; \
	    helpMessage = substr(lastLine, RSTART + 3, RLENGTH); \
      gsub("\\\\", "", helpCommand); \
      gsub(":+$$", "", helpCommand); \
	    printf "  \x1b[32;01m%-35s\x1b[0m %s\n", helpCommand, helpMessage; \
	  } \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST) | sort -u
	@printf "\n"
	@printf "CLI Usage:\n\n"
	@bash +x scripts/help.sh