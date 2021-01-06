package service

import (
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"redis-sentinel/pkg/k8s"

	rsv1"redis-sentinel/api/v1"
	"redis-sentinel/pkg/util"
)

// RedisClusterClient has the minimumm methods that a Redis cluster controller needs to satisfy
// in order to talk with K8s
type RedisSentinelClient interface {
	EnsureSentinelService(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) error
	EnsureSentinelHeadlessService(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) error
	EnsureSentinelConfigMap(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) error
	EnsureSentinelProbeConfigMap(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) error
	EnsureSentinelStatefulset(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) error
	EnsureRedisStatefulset(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) error
	EnsureRedisService(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) error
	EnsureRedisShutdownConfigMap(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) error
	EnsureRedisConfigMap(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) error
	EnsureNotPresentRedisService(rs *rsv1.RedisSentinel) error
}

// RedisClusterKubeClient implements the required methods to talk with kubernetes
type RedisSentinelKubeClient struct {
	K8SService k8s.Services
	logger     logr.Logger
}

// NewRedisClusterKubeClient creates a new RedisClusterKubeClient
func NewRedisClusterKubeClient(k8sService k8s.Services, logger logr.Logger) *RedisSentinelKubeClient {
	return &RedisSentinelKubeClient{
		K8SService: k8sService,
		logger:     logger,
	}
}

func generateSelectorLabels(component, name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/part-of":   util.AppLabel,
		"app.kubernetes.io/component": component,
		"app.kubernetes.io/name":      name,
	}
}

// EnsureSentinelService makes sure the sentinel service exists
func (r *RedisSentinelKubeClient) EnsureSentinelService(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) error {
	svc := generateSentinelService(rs, labels, ownerRefs)
	return r.K8SService.CreateIfNotExistsService(rs.Namespace, svc)
}

// EnsureSentinelHeadlessService makes sure the sentinel headless service exists
func (r *RedisSentinelKubeClient) EnsureSentinelHeadlessService(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) error {
	svc := newHeadLessSvcForCR(rs, labels, ownerRefs)
	return r.K8SService.CreateIfNotExistsService(rs.Namespace, svc)
}

// EnsureSentinelConfigMap makes sure the sentinel configmap exists
func (r *RedisSentinelKubeClient) EnsureSentinelConfigMap(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) error {
	cm := generateSentinelConfigMap(rs, labels, ownerRefs)
	return r.K8SService.CreateIfNotExistsConfigMap(rs.Namespace, cm)
}

// EnsureSentinelConfigMap makes sure the sentinel configmap exists
func (r *RedisSentinelKubeClient) EnsureSentinelProbeConfigMap(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) error {
	cm := generateSentinelReadinessProbeConfigMap(rs, labels, ownerRefs)
	return r.K8SService.CreateIfNotExistsConfigMap(rs.Namespace, cm)
}

// EnsureSentinelStatefulset makes sure the sentinel deployment exists in the desired state
func (r *RedisSentinelKubeClient) EnsureSentinelStatefulset(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) error {
	if err := r.ensurePodDisruptionBudget(rs, util.SentinelName, util.SentinelRoleName, labels, ownerRefs); err != nil {
		return err
	}

	oldSs, err := r.K8SService.GetStatefulSet(rs.Namespace, util.GetSentinelName(rs))
	if err != nil {
		// If no resource we need to create.
		if errors.IsNotFound(err) {
			ss := generateSentinelStatefulSet(rs, labels, ownerRefs)
			return r.K8SService.CreateStatefulSet(rs.Namespace, ss)
		}
		return err
	}

	if shouldUpdateRedis(rs.Spec.Sentinel.Resources, oldSs.Spec.Template.Spec.Containers[0].Resources, rs.Spec.Sentinel.Replicas, *oldSs.Spec.Replicas) {
		ss := generateSentinelStatefulSet(rs, labels, ownerRefs)
		return r.K8SService.UpdateStatefulSet(rs.Namespace, ss)
	}
	return nil
}

// EnsureRedisStatefulset makes sure the redis statefulset exists in the desired state
func (r *RedisSentinelKubeClient) EnsureRedisStatefulset(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) error {
	if err := r.ensurePodDisruptionBudget(rs, util.RedisName, util.RedisRoleName, labels, ownerRefs); err != nil {
		return err
	}

	oldSs, err := r.K8SService.GetStatefulSet(rs.Namespace, util.GetRedisName(rs))
	if err != nil {
		// If no resource we need to create.
		if errors.IsNotFound(err) {
			ss := generateRedisStatefulSet(rs, labels, ownerRefs)
			return r.K8SService.CreateStatefulSet(rs.Namespace, ss)
		}
		return err
	}

	if shouldUpdateRedis(rs.Spec.Resources, oldSs.Spec.Template.Spec.Containers[0].Resources,
		rs.Spec.Size, *oldSs.Spec.Replicas) || exporterChanged(rs, oldSs) {
		ss := generateRedisStatefulSet(rs, labels, ownerRefs)
		return r.K8SService.UpdateStatefulSet(rs.Namespace, ss)
	}

	return nil
}

func exporterChanged(rs *rsv1.RedisSentinel, sts *appsv1.StatefulSet) bool {
	if rs.Spec.Exporter.Enabled {
		for _, container := range sts.Spec.Template.Spec.Containers {
			if container.Name == exporterContainerName {
				return false
			}
		}
		return true
	} else {
		for _, container := range sts.Spec.Template.Spec.Containers {
			if container.Name == exporterContainerName {
				return true
			}
		}
		return false
	}
}

func shouldUpdateRedis(expectResource, containterResource corev1.ResourceRequirements, expectSize, replicas int32) bool {
	if expectSize != replicas {
		return true
	}
	if result := containterResource.Requests.Cpu().Cmp(*expectResource.Requests.Cpu()); result != 0 {
		return true
	}
	if result := containterResource.Requests.Memory().Cmp(*expectResource.Requests.Memory()); result != 0 {
		return true
	}
	if result := containterResource.Limits.Cpu().Cmp(*expectResource.Limits.Cpu()); result != 0 {
		return true
	}
	if result := containterResource.Limits.Memory().Cmp(*expectResource.Limits.Memory()); result != 0 {
		return true
	}
	return false
}

// EnsureRedisConfigMap makes sure the sentinel configmap exists
func (r *RedisSentinelKubeClient) EnsureRedisConfigMap(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) error {
	cm := generateRedisConfigMap(rs, labels, ownerRefs)
	return r.K8SService.CreateIfNotExistsConfigMap(rs.Namespace, cm)
}

// EnsureRedisShutdownConfigMap makes sure the redis configmap with shutdown script exists
func (r *RedisSentinelKubeClient) EnsureRedisShutdownConfigMap(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) error {
	if rs.Spec.ShutdownConfigMap != "" {
		if _, err := r.K8SService.GetConfigMap(rs.Namespace, rs.Spec.ShutdownConfigMap); err != nil {
			return err
		}
	} else {
		cm := generateRedisShutdownConfigMap(rs, labels, ownerRefs)
		return r.K8SService.CreateIfNotExistsConfigMap(rs.Namespace, cm)
	}
	return nil
}

// EnsureRedisService makes sure the redis statefulset exists
func (r *RedisSentinelKubeClient) EnsureRedisService(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) error {
	svc := generateRedisService(rs, labels, ownerRefs)
	return r.K8SService.CreateIfNotExistsService(rs.Namespace, svc)
}

// EnsureNotPresentRedisService makes sure the redis service is not present
func (r *RedisSentinelKubeClient) EnsureNotPresentRedisService(rs *rsv1.RedisSentinel) error {
	name := util.GetRedisName(rs)
	namespace := rs.Namespace
	// If the service exists (no get error), delete it
	if _, err := r.K8SService.GetService(namespace, name); err == nil {
		return r.K8SService.DeleteService(namespace, name)
	}
	return nil
}

// EnsureRedisStatefulset makes sure the pdb exists in the desired state
func (r *RedisSentinelKubeClient) ensurePodDisruptionBudget(rs *rsv1.RedisSentinel, name string, component string, labels map[string]string, ownerRefs []metav1.OwnerReference) error {
	name = util.GenerateName(name, rs.Name)
	namespace := rs.Namespace

	minAvailable := intstr.FromInt(2)
	labels = util.MergeLabels(labels, generateSelectorLabels(component, rs.Name))

	pdb := generatePodDisruptionBudget(name, namespace, labels, ownerRefs, minAvailable)

	return r.K8SService.CreateIfNotExistsPodDisruptionBudget(namespace, pdb)
}
