# syntax=docker/dockerfile:1
# https://docs.docker.com/engine/reference/builder/#buildkit

FROM scratch as base
ARG TARGETOS
ARG TARGETARCH
ADD bin/oci-lb-ingress-controller-${TARGETOS}-${TARGETARCH} /app/oci-lb-ingress-controller
ADD config/config.default.yml /app/config.yml

FROM alpine:latest

COPY --from=base /app /app

WORKDIR /app
ENTRYPOINT ["/app/oci-lb-ingress-controller"]
CMD [ "-config", "/app/config.yml" ] 