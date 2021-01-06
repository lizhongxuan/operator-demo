package k8s

import (
	"context"
	"github.com/go-logr/logr"
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
	rs.Status.DescConditionsByTime()
	err := c.client.Status().Update(context.TODO(), rs)
	if err != nil {
		c.logger.WithValues("namespace", namespace, "cluster", rs.Name, "conditions", rs.Status.Conditions).
			Error(err, "redisClusterStatus")
		return err
	}
	c.logger.WithValues("namespace", namespace, "cluster", rs.Name, "conditions", rs.Status.Conditions).
		V(3).Info("redisClusterStatus updated")
	return nil
}
