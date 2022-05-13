
# OCI LB Ingress Controller

## Relationships

```plain
<->  : one to one
*->  : many to one
<-*  : one to many
*-*  : many to many
%    : association site (which affects deletion/creation order)
~    : variant
+    : another attribute to account
|    : alternative
?    : optional
```

- `Listener <-> %RoutingPolicy   (oci relation: *-*)`
- `Listener% <-> HostnameName    (oci relation: *-*)`
- `Listener% *-> BackendSet      (oci relation: *-1  | but more could be associated using routing policies)`
- `Listener% ~tls *-> Certificate`

- `k8s::Service{.name + .port} <-> BackendSet`
- `k8s::Secret  <-> Certificate`
- `k8s::Ingress::Rule[].Host ~tls *-> Certificate`
- `k8s::Ingress::Rule[].Host <-> RoutingPolicy`
- `k8s::Ingress::Rule[].Host <-> HostnameName`
- `k8s::Ingress::Rule[].Host+protocol+port <-> Listener`
assure-recorder-bac-T7000-AxCHDz

## Resource Naming

| Name                  | Length   | Regex                           | Derived as                                                                                                                                                                    | Examples                                   |
| --------------------- | -------- | ------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------ |
| LB DisplayName        | [1-1024] | `^[a-zA-Z0-9_-]{1,1024}$`       | optionalPrefix + k8s::Ingress {.namespace + "_" + .name} }                                                                                                                    |                                            |
| BackendSetName        | [1-32]   | `^[a-zA-Z0-9_-]{1,32}$`         | k8s::Service{.name[0:26] + "*" + .protocol as "T"/"U" + .port  + "*" +  digestPadding(.namespace+.name, len=\|32\|) }  <br> "default"                                         | nginx_T8080_aH5RtfhfAgghOr6WfhEn           |
| hostnameName          | [1-255]  |                                 | k8s::Ingress::rule[].host                                                                                                                                                     |                                            |
| ListenerName          | [1-255]  | `^[a-zA-Z0-9_-]{1,255}$`        | replace(k8s::Ingress::rule[].host,"*."->"STAR", "."->"DOT") + optionalDigestPadding(.host, \|240-255\|)  <br>  "http-to-https-redirector"  <br>  "default" if .hostname == "" | wwwDOTexampleDOTcom <br> STARexampleDOTcom |
| RoutingPolicyName     | [1-32]   | `^[a-zA-Z_][a-zA-Z0-9_]{1,31}$` | replace(k8s::Ingress::rule[].host, "*." -> "S_","-" -> "*" , "." -> "*")  +  digestPadding(.host, len=\|32\|)                                                                 | www_example_comj3LykQwVlJWmElxfS           |
| RoutingPolicyRuleName | [1-32]   | `^[a-zA-Z_][a-zA-Z0-9_]{1,31}$` | digest(rule, len=32)                                                                                                                                                          | YzIVv0b4SalAWS0c5RaShA                     |
| RuleSetName           | [1-32]   | `^[a-zA-Z_][a-zA-Z0-9_]{0,31}$` | "https_301_redirection"                                                                                                                                                       |                                            |
| CertificateName       | [1-255]  |                                 | k8s::Ingress::tls.secretName + digest of x509 signature                                                                                                                       |                                            |
| ~~PathRouteSets~~     | -        |                                 | -                                                                                                                                                                             |                                            |

## Other OCI Restrictions and shenanigans

- Certificate is required for HTTP/2 Listener (And of course for HTTPS listener too)
- As of now, HTTP2 listener can only support a default cipher suite 'oci-default-http2-ssl-cipher-suit'

## Integrations

- `kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.5.3/cert-manager.yaml`

# Links

<https://github.com/kubernetes/cloud-provider/blob/0203c3ea624efbed17a9d0549f931eb94393b47b/cloud.go#L133>

# NOTE

Kubernetes 1.19 introduced a new networking.k8s.io/v1 API for the Ingress resource. It standardizes common practices and clarifies implementation requirements that were previously up to individual controller vendors.

[This document covers those changes as they relate to Kubernetes Ingress](https://docs.konghq.com/kubernetes-ingress-controller/1.3.x/concepts/ingress-versions/)

See: [Nginx controller migrates to new networking.k8s.io/v1beta1 package](https://github.com/kubernetes/ingress-nginx/commit/84102eec2ba270f624c57023aab59aab4471178e)

## OCI K8s support

<https://docs.oracle.com/en-us/iaas/Content/ContEng/Concepts/contengaboutk8sversions.htm>
1.21.5  Yes  Support introduced: December 9, 2021
1.20.11  Yes  Support introduced: October 8, 2021
1.20.8  Yes, until November 7, 2021  Support introduced: July 20, 2021

Note that although Kubernetes version 1.20.8 will not be supported after November 7, 2021, it will continue to be available for selection. However, Oracle strongly recommends you upgrade clusters to Kubernetes version 1.20.11.
1.19.15  Yes  Support introduced: October 8, 2021
1.19.12  Yes, until November 7, 2021  Support introduced: July 13, 2021

Note that although Kubernetes version 1.19.12 will not be supported after November 7, 2021, it will continue to be available for selection. However, Oracle strongly recommends you upgrade clusters to Kubernetes version 1.19.15.
1.18.10  Yes, until February 9th, 2022  Support introduced: 1 December, 2020

Oracle strongly recommends you upgrade clusters to Kubernetes version 1.21.5, 1.20.11, or 1.19.15
