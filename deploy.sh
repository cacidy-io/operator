#!/bin/sh
IMG=ttl.sh/$(uuidgen):1h
make docker-build IMG=$IMG
make docker-push IMG=$IMG
make deploy IMG=$IMG
