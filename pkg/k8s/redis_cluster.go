package k8s

import (
	"context"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	rsv1 "redis-sentinel/api/v1"
)

// Cluster the client that knows how to interact with kubernetes to manage RedisCluster
type Cluster interface {
	// UpdateCluster update the RedisCluster
	UpdateCluster(namespace string, rs *rsv1.RedisSentinel) error
}

// ClusterOption is the RedisCluster client that using API calls to kubernetes.
type ClusterOption struct {
	client client.Client
	logger logr.Logger
}

// NewCluster returns a new RedisCluster client.
func NewCluster(kubeClient client.Client, logger logr.Logger) Cluster {
	logger = logger.WithValues("service", "crd.redisCluster")
	return &ClusterOption{
		client: kubeClient,
		logger: logger,
	}
}

// UpdateCluster implement the  Cluster.Interface
func (c *ClusterOption) UpdateCluster(namespace string, rs *rsv1.RedisSentinel) error {
	instance := &rsv1.RedisSentinel{}
	if err := c.client.Get(context.TODO(), types.NamespacedName{
		Namespace: rs.Namespace,
		Name: rs.Name,
	}, instance);err != nil{
		c.logger.WithValues("namespace", rs.Namespace, "cluster", rs.Name).
			Error(err, "RedisSentinel.GET")
		return err
	}

	instance.Labels = instance.Labels
	instance.Annotations = instance.Annotations
	instance.Spec = instance.Spec
	instance.ClusterName = instance.ClusterName
	
	err := c.client.Update(context.TODO(), instance)
	if err != nil {
		c.logger.WithValues("namespace", namespace, "cluster", rs.Name, "conditions", rs.Status.Conditions).
			Error(err, "redisClusterStatus")
		return err
	}
	c.logger.WithValues("namespace", namespace, "cluster", rs.Name, "conditions", rs.Status.Conditions).
		V(3).Info("redisClusterStatus updated")
	return nil
}
