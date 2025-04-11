# [Choice] Go version (use -bullseye variants on local arm64/Apple Silicon): 1, 1.18, 1.17, 1-bullseye, 1.18-bullseye, 1.17-bullseye, 1-buster, 1.18-buster, 1.17-buster
FROM mcr.microsoft.com/vscode/devcontainers/go:1-1.24-bullseye

RUN go version

# [Choice] Node.js version: none, lts/*, 16, 14, 12, 10
ARG NODE_VERSION="none"
RUN if [ "${NODE_VERSION}" != "none" ]; then su vscode -c "umask 0002 && . /usr/local/share/nvm/nvm.sh && nvm install ${NODE_VERSION} 2>&1"; fi

RUN export DEBIAN_FRONTEND=noninteractive \
    && curl https://baltocdn.com/helm/signing.asc | gpg --dearmor | tee /usr/share/keyrings/helm.gpg \
    &&  curl -sS https://dl.yarnpkg.com/debian/pubkey.gpg | gpg --dearmor > /usr/share/keyrings/yarn-archive-keyring.gpg \
    && apt-get install apt-transport-https --yes \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/helm.gpg] https://baltocdn.com/helm/stable/debian/ all main" | tee /etc/apt/sources.list.d/helm-stable-debian.list \
    && apt-get update \
    && apt-get install --yes --no-install-recommends \
        bash-completion \
        helm \
        uuid-runtime

ARG CRDB_VERSION=v24.1.6
# Install cockroachdb so we have the client
RUN curl "https://binaries.cockroachdb.com/cockroach-${CRDB_VERSION}.linux-$(go env GOARCH).tgz" | tar -xz \
    && cp -i "cockroach-${CRDB_VERSION}.linux-$(go env GOARCH)/cockroach" /usr/local/bin/ \
    && rm -rf cockroach-v*

ENV COCKROACH_BINARY=/usr/local/bin/cockroach

USER vscode

# Add ~/.ssh for the vscode user so updating known_hosts works and
# add .devcontainer/.tools to PATH
RUN mkdir --mode=700 /home/vscode/.ssh && \
    echo 'export PATH="${PATH}:/workspace/.tools"' >> ~/.bashrc

WORKDIR /workspace
