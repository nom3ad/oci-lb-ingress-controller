GOOS?=linux
GOARCH?=amd64

PKG=github.com/nom3ad/oci-lb-ingress-controller
TAG ?= $(shell cat TAG)
REPO_INFO ?= $(shell git config --get remote.origin.url || basename $$(pwd))
COMMIT_SHA ?= git-$(shell git rev-parse --short HEAD)
BUILD_TIMESTAMP ?= $(shell date --utc --iso-8601=minutes)

BUILD_OUTPUT?=bin/oci-lb-ingress-controller-$(GOOS)-$(GOARCH)

K8S_SDK_VERSION=$(shell grep -oP 'k8s.io/api \K.*'  go.mod || echo UNKNOWN)
OCI_SDK_VERSION=$(shell grep -oP 'oracle/oci-go-sdk.* \K.*'  go.mod  || echo UNKNOWN)

LDFLAGS=-X $(PKG)/version.RELEASE_VERSION=$(TAG) -X $(PKG)/version.COMMIT=$(COMMIT_SHA) -X $(PKG)/version.BUILD_TIMESTAMP=$(BUILD_TIMESTAMP) -X $(PKG)/version.REPO=$(REPO_INFO) -X $(PKG)/version.OCI_SDK_VERSION=$(OCI_SDK_VERSION) -X $(PKG)/version.K8S_SDK_VERSION=$(K8S_SDK_VERSION)

all: build

.PHONY: build
build:
	@set -e; \
	GOOS=$(GOOS) \
	GOARCH=$(GOARCH) \
	CGO_ENABLED=0 \
	go build -v -ldflags="-s -w $(LDFLAGS)" -o $(BUILD_OUTPUT) ./cmd/oci-lb-ingress-controller/ && \
	file $(BUILD_OUTPUT); ls -alh $(BUILD_OUTPUT); 

.PHONY: run
run:
	@set -e; \
	args=${args:-'-config ./config.yml -ingress-class oci -kubeconfig ~/.kube/config'}\
	CGO_ENABLED=0 \
	ZAP_DEV_LOGGER=true \
	ZAP_LOG_LEVEL=debug \
	go run -ldflags="$(LDFLAGS)" cmd/oci-lb-ingress-controller/main.go $$args

.PHONY: image
image: build
	@set -e; \
	tag=$${tag:-oci-lb-ingress-controller}; \
	docker build --build-arg TARGETOS=$(GOOS) --build-arg TARGETARCH=$(GOARCH)  -t $$tag -f Dockerfile .; \
	read -p "Push (Y/n)?" && [[ $${REPLY,} == "y" ]] && docker push $$tag;

.PHONY: build-and-push-multi-arch-image
build-and-push-multi-arch-image:  
	@set -e; \
	make build GOOS=linux GOARCH=arm64;\
	make build GOOS=linux GOARCH=amd64;\
	tag=$${tag-oci-lb-ingress-controller}; \
	docker buildx build --push --platform linux/arm64,linux/amd64 -t $$tag -f Dockerfile .;


.PHONY: deploy-ingress-controller
deploy-ingress-controller:
	kubectl apply -f manifests/ingress-controller.yml

.PHONY: deploy-ingress-example
deploy-ingress-example:
	kubectl apply -f manifests/ingress-example.yml

.PHONY: delete-ingress-example
delete-ingress-example:
	kubectl delete -f manifests/ingress-example.yml

.PHONY: logs-ingress-controller
logs-ingress-controller:
	kubectl logs deployment/oci-lb-ingress-controller -f

.PHONY: update-example-tls-certs
update-example-tls-certs:
	mkdir -p tmp && cd tmp && \
	openssl req -x509 -nodes -days $$((365 * 4)) -newkey rsa:2048 -keyout tls.key -out tls.crt -subj "/CN=*.example.test/O=oci-lb-ingress-example" && \
	kubectl create secret tls ingress-example-tls-cert  --key tls.key --cert tls.crt --save-config --namespace=oci-lb-ingress-example --dry-run=client -o yaml | tee | kubectl apply -f -