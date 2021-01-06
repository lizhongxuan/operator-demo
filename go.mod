module redis-sentinel

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/go-openapi/spec v0.20.0 // indirect
	github.com/go-redis/redis v6.15.2+incompatible
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/prometheus/client_golang v1.0.0
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.0.0-20191016110408-35e52d86657a
	k8s.io/apimachinery v0.0.0-20191004115801-a2eda9f80ab8
	k8s.io/client-go v0.0.0-20191016111102-bec269661e48
	sigs.k8s.io/controller-runtime v0.4.0
)
