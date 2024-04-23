#!/bin/sh

set -e

function clone_app() {
    app_auth=""
    if [ ! -z $APP_PASSWORD ]; then app_auth="${APP_USERNAME}:${APP_PASSWORD}@"; fi
    git config --global --add safe.directory /src
    git clone --quiet --depth 1 ${APP_URL/\/\////$app_auth} /src && cd /src
    git fetch --quiet --depth 1 origin ${APP_REVISION}
    git checkout --quiet ${APP_REVISION}
}

function clone_module() {
    module_auth=""
    if [ ! -z $MODULE_PASSWORD ]; then module_auth="${MODULE_USERNAME}:${MODULE_PASSWORD}@"; fi
    git config --global --add safe.directory /module
    git clone --quiet --depth 1 ${MODULE_URL/\/\////$module_auth} /module && cd /module
    git fetch --quiet --depth 1 origin ${MODULE_REVISION}
    git checkout --quiet ${MODULE_REVISION}
}

function call() {
    export _EXPERIMENTAL_DAGGER_RUNNER_HOST=kube-pod://${DAGGER_ENGINE_POD_NAME}?namespace=${DAGGER_ENGINE_NAMESPACE}
    dagger call --source /src ${MODULE_FUNCTION_ARGS} ${MODULE_FUNCTION}
}

function main() {
    echo "Cacidy Runner (v0.1.0-alpha)"
    echo ""
    echo -e "\e[32mApplication:\e[0m ${APP_URL}:${APP_BRANCH}" 
    echo -e "\e[32mModule:\e[0m      ${MODULE_URL}/commit/${MODULE_REVISION}"
    echo -e "\e[32mFunction:\e[0m    ${MODULE_FUNCTION}"
    echo -e "\e[32mEngine:\e[0m      ${DAGGER_ENGINE_NAMESPACE}/${DAGGER_ENGINE_POD_NAME}"
    echo ""
    echo "starting pipeline..."
    clone_app
    clone_module
    call
}

main
