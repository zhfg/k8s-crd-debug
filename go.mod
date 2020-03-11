module github.com/zhfg/k8s-crd-debug

go 1.13

replace k8s.io/client-go => github.com/kubernetes/client-go v0.0.0-20200228043304-076fbc5c36a7

require (
	github.com/golang/protobuf v1.3.4 // indirect
	github.com/imdario/mergo v0.3.8 // indirect
	golang.org/x/crypto v0.0.0-20200302210943-78000ba7a073 // indirect
	golang.org/x/net v0.0.0-20200301022130-244492dfa37a // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	k8s.io/api v0.17.3
	k8s.io/apimachinery v0.17.3
	k8s.io/client-go v0.0.0-20200228043759-e3bfc0127563
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20200229041039-0a110f9eb7ab // indirect
)
