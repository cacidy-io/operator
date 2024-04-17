FROM alpine
ENV DAGGER_VERSION=0.11.0

RUN apk add \
    curl \
    docker \
    git \
    kubectl

RUN adduser --disabled-password cacidy
RUN mkdir /module /src
RUN chmod 777 /module /src

USER cacidy

RUN curl -L https://dl.dagger.io/dagger/install.sh | BIN_DIR=$HOME/.local/bin sh
ENV PATH=$PATH:/home/cacidy/.local/bin

COPY docker-entrypoint.sh /home/cacidy/.local/bin/docker-entrypoint

ENTRYPOINT ["docker-entrypoint"]
