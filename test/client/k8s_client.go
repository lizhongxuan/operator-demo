package client

import (
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"redis-sentinel/api/v1"
)

func NewK8sClient(config *rest.Config) (client.Client, error) {
	scheme := scheme.Scheme
	// AddToScheme 改成非redissentinel的
	err := v1.AddToScheme(scheme)

	mapper, err := apiutil.NewDiscoveryRESTMapper(config)
	if err != nil {
		return nil, err
	}
	options := client.Options{
		Scheme: scheme,
		Mapper: mapper,
	}
	cli, err := client.New(config, options)
	if err != nil {
		return nil, err
	}
	return cli, nil
}
