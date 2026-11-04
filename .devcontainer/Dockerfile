FROM debian:trixie-slim

ARG GO_VERSION=1.27.3
ARG NODE_VERSION=24.22.2

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git \
    postgresql-client \
    xz-utils \
    && curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xz \
    && curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-x64.tar.xz" | tar -C /usr/local --strip-components=1 -xJf - \
    && rm -rf /var/lib/apt/lists/*

ENV PATH="/usr/local/go/bin:/usr/local/bin:/home/vscode/go/bin:${PATH}" \
    GOPATH="/home/vscode/go"

RUN npx -y playwright@1 install-deps chromium && rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash vscode && mkdir -p /workspace && chown -R vscode:vscode /workspace /home/vscode/go

USER vscode
WORKDIR /workspace

ENV PATH="/home/vscode/go/bin:/usr/local/go/bin:/usr/local/bin:${PATH}"

RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest && \
    go install github.com/pressly/goose/v3/cmd/goose@latest && \
    go install golang.org/x/vuln/cmd/govulncheck@latest && \
    go install github.com/air-verse/air@latest && \
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest && \
    go install golang.org/x/tools/gopls@latest

CMD ["/bin/bash"]
