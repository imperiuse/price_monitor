ARG GOLANG_IMAGE
ARG ALPINE_IMAGE

## GOLANG BUILD STAGE##
FROM $GOLANG_IMAGE as build-env

ARG APP_ENV
ARG APP_NAME
ARG APP_VERSION
ARG APP_NODE_NAME

ENV APP_ENV=$APP_ENV
ENV APP_NODE_NAME=$APP_NODE_NAME
ENV APP_NAME=$APP_NAME
ENV APP_VERSION=$APP_VERSION

WORKDIR /src

COPY ["go.mod", "go.sum", "./"]

RUN go mod download

COPY Makefile .
COPY configs  ./configs
COPY cmd  ./cmd
COPY internal  ./internal
COPY tools  ./tools
COPY migrations  ./migrations

RUN : \
    && mkdir -p /dist                                                    \
    && make build                                                        \
    && mv -v ./bin/${APP_NAME} /dist

## APP STAGE ##
FROM $ALPINE_IMAGE as app

ARG APP_NAME
ENV APP_NAME=$APP_NAME

RUN : mkdir -p /opt/${APP_NAME}/                  \
               /opt/${APP_NAME}/configs

COPY --from=build-env /dist/${APP_NAME}    /opt/${APP_NAME}/${APP_NAME}
COPY --from=build-env /src/configs         /opt/${APP_NAME}/configs

# fix x509 error  https://stackoverflow.com/questions/52969195/docker-container-running-golang-http-client-getting-error-certificate-signed-by
COPY --from=build-env /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

WORKDIR /opt/${APP_NAME}
ENTRYPOINT ["./${APP_NAME}"]
