# syntax=docker/dockerfile:1

FROM alpine:latest

ARG TARGETOS
ARG TARGETARCH
# https://docs.docker.com/engine/reference/builder/#buildkit

ADD bin/oci-lb-ingress-controller-${TARGETOS}-${TARGETARCH} /app/oci-lb-ingress-controller
ADD config/config.default.yml /app/config.yml

WORKDIR /app
ENTRYPOINT ["/app/oci-lb-ingress-controller"]
CMD [ "-config", "/app/config.yml" ] 