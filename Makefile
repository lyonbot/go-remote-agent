SESSION := go-remote-agent

.PHONY: build dev

build:
	cd frontend-src && pnpm install && pnpm build
	GOARCH=amd64 GOOS=linux go build -ldflags "-s -w" -tags release -o agent_linux_amd64 .

dev:
	cd frontend-src && pnpm install
	@if tmux has-session -t $(SESSION) 2>/dev/null; then \
		tmux send-keys -t $(SESSION):0.0 C-c "" 2>/dev/null; \
		tmux send-keys -t $(SESSION):0.1 C-c "" 2>/dev/null; \
		tmux send-keys -t $(SESSION):0.2 C-c "" 2>/dev/null; \
	else \
		tmux new-session -d -s $(SESSION); \
		tmux split-window -h -t $(SESSION):0; \
		tmux split-window -v -t $(SESSION):0.1; \
	fi
	@tmux send-keys -t $(SESSION):0.0 "go run main.go" Enter
	@tmux send-keys -t $(SESSION):0.2 "cd frontend-src && pnpm dev" Enter
	@sleep 1
	@tmux send-keys -t $(SESSION):0.1 "go run main.go -a" Enter
	echo $$ tmux attach-session -t $(SESSION)
