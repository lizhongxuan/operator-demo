package controllers


import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rsv1 "redis-sentinel/api/v1"
)

// Ensure the RedisCluster's components are correct.
func (rsh *RedisSentinelHandler) Ensure(rs *rsv1.RedisSentinel, labels map[string]string, or []metav1.OwnerReference) error {
	if err := rsh.rcService.EnsureRedisService(rs, labels, or); err != nil {
		return err
	}
	if err := rsh.rcService.EnsureSentinelService(rs, labels, or); err != nil {
		return err
	}
	if err := rsh.rcService.EnsureSentinelHeadlessService(rs, labels, or); err != nil {
		return err
	}
	if err := rsh.rcService.EnsureSentinelConfigMap(rs, labels, or); err != nil {
		return err
	}
	if err := rsh.rcService.EnsureSentinelProbeConfigMap(rs, labels, or); err != nil {
		return err
	}
	if err := rsh.rcService.EnsureRedisShutdownConfigMap(rs, labels, or); err != nil {
		return err
	}
	if err := rsh.rcService.EnsureRedisStatefulset(rs, labels, or); err != nil {
		return err
	}
	if err := rsh.rcService.EnsureSentinelStatefulset(rs, labels, or); err != nil {
		return err
	}

	return nil
}
