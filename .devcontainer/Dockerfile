FROM mcr.microsoft.com/devcontainers/go:1-1.21-bookworm

RUN curl -L -o kubebuilder "https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)" && \
    sudo chmod +x kubebuilder && \
    sudo install kubebuilder /usr/local/bin/ && \
    rm kubebuilder

RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    chmod +x kubectl && \
    sudo install kubectl /usr/local/bin/kubectl && \
    rm kubectl

RUN sudo apt update && sudo apt install vim -y