package handle


import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rsv1 "redis-sentinel/api/v1"
)

// Ensure the RedisCluster's components are correct.
func (rsh *RedisSentinelHandler) Ensure(rs *rsv1.RedisSentinel, labels map[string]string, or []metav1.OwnerReference) error {
	if err := rsh.RsService.EnsureRedisService(rs, labels, or); err != nil {
		return err
	}
	if err := rsh.RsService.EnsureSentinelService(rs, labels, or); err != nil {
		return err
	}
	if err := rsh.RsService.EnsureSentinelHeadlessService(rs, labels, or); err != nil {
		return err
	}
	if err := rsh.RsService.EnsureSentinelConfigMap(rs, labels, or); err != nil {
		return err
	}
	if err := rsh.RsService.EnsureSentinelProbeConfigMap(rs, labels, or); err != nil {
		return err
	}
	if err := rsh.RsService.EnsureRedisShutdownConfigMap(rs, labels, or); err != nil {
		return err
	}
	if err := rsh.RsService.EnsureRedisStatefulset(rs, labels, or); err != nil {
		return err
	}
	if err := rsh.RsService.EnsureSentinelStatefulset(rs, labels, or); err != nil {
		return err
	}

	return nil
}
