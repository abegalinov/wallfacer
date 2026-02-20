SHELL   := /bin/bash
PODMAN  := /opt/podman/bin/podman
IMAGE   := wallfacer:latest
NAME    := wallfacer

# Load .env if it exists
-include .env
export

.PHONY: build server run shell clean

# Build the sandbox image
build:
	$(PODMAN) build -t $(IMAGE) -f sandbox/Dockerfile sandbox/

# Build and run the Go server natively
server:
	go build -o wallfacer . && ./wallfacer run

# Space-separated list of folders to mount under /workspace/<basename>
WORKSPACES ?= $(CURDIR)

# Generate -v flags: /path/to/foo -> -v /path/to/foo:/workspace/foo:z
VOLUME_MOUNTS := $(foreach ws,$(WORKSPACES),-v $(ws):/workspace/$(notdir $(ws)):z)

# Headless one-shot: make run PROMPT="fix the failing tests"
run:
ifndef PROMPT
	$(error PROMPT is required. Usage: make run PROMPT="your task here")
endif
	@$(PODMAN) run --rm -it \
		--name $(NAME) \
		--env-file .env \
		$(VOLUME_MOUNTS) \
		-v claude-config:/home/claude/.claude \
		-w /workspace \
		$(IMAGE) -p "$(PROMPT)" --verbose --output-format stream-json

# Debug shell into a sandbox container
shell:
	$(PODMAN) run --rm -it \
		--name $(NAME)-shell \
		--env-file .env \
		$(VOLUME_MOUNTS) \
		-v claude-config:/home/claude/.claude \
		-w /workspace \
		--entrypoint /bin/bash \
		$(IMAGE)

# Remove the sandbox image
clean:
	-$(PODMAN) rmi $(IMAGE)
