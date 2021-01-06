package service

import (
	"fmt"
	"strings"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	rsv1 "redis-sentinel/api/v1"
	"redis-sentinel/pkg/util"
	)

const (
	redisShutdownConfigurationVolumeName = "redis-shutdown-config"
	redisStorageVolumeName               = "redis-data"

	graceTime = 30
)

func generateSentinelService(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) *corev1.Service {
	name := util.GetSentinelName(rs)
	namespace := rs.Namespace

	sentinelTargetPort := intstr.FromInt(26379)
	labels = util.MergeLabels(labels, generateSelectorLabels(util.SentinelRoleName, rs.Name))

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          labels,
			OwnerReferences: ownerRefs,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "sentinel",
					Port:       26379,
					TargetPort: sentinelTargetPort,
					Protocol:   "TCP",
				},
			},
		},
	}
}

func generateRedisService(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) *corev1.Service {
	name := util.GetRedisName(rs)
	namespace := rs.Namespace

	labels = util.MergeLabels(labels, generateSelectorLabels(util.RedisRoleName, rs.Name))
	redisTargetPort := intstr.FromInt(6379)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          labels,
			OwnerReferences: ownerRefs,
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: corev1.ClusterIPNone,
			Ports: []corev1.ServicePort{
				{
					Port:       6379,
					Protocol:   corev1.ProtocolTCP,
					Name:       "redis",
					TargetPort: redisTargetPort,
				},
			},
			Selector: labels,
		},
	}
}

func generateSentinelConfigMap(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) *corev1.ConfigMap {
	name := util.GetSentinelName(rs)
	namespace := rs.Namespace

	labels = util.MergeLabels(labels, generateSelectorLabels(util.SentinelRoleName, rs.Name))
	sentinelConfigFileContent := `sentinel monitor mymaster 127.0.0.1 6379 2
sentinel down-after-milliseconds mymaster 1000
sentinel failover-timeout mymaster 3000
sentinel parallel-syncs mymaster 2`

	if rs.Spec.Password != "" {
		sentinelConfigFileContent = fmt.Sprintf("%s\nsentinel auth-pass mymaster %s\n", sentinelConfigFileContent, rs.Spec.Password)
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          labels,
			OwnerReferences: ownerRefs,
		},
		Data: map[string]string{
			util.SentinelConfigFileName: sentinelConfigFileContent,
		},
	}
}

func generateRedisConfigMap(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) *corev1.ConfigMap {
	name := util.GetRedisName(rs)
	namespace := rs.Namespace

	labels = util.MergeLabels(labels, generateSelectorLabels(util.RedisRoleName, rs.Name))
	redisConfigFileContent := `slaveof 127.0.0.1 6379
tcp-keepalive 60
save 900 1
save 300 10`
	if rs.Spec.Password != "" {
		redisConfigFileContent = fmt.Sprintf("%s\nrequirepass %s\nmasterauth %s\n", redisConfigFileContent, rs.Spec.Password, rs.Spec.Password)
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          labels,
			OwnerReferences: ownerRefs,
		},
		Data: map[string]string{
			util.RedisConfigFileName: redisConfigFileContent,
		},
	}
}

func generateRedisShutdownConfigMap(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) *corev1.ConfigMap {
	name := util.GetRedisShutdownConfigMapName(rs)
	namespace := rs.Namespace

	labels = util.MergeLabels(labels, generateSelectorLabels(util.RedisRoleName, rs.Name))
	envSentinelHost := fmt.Sprintf("REDIS_SENTINEL_%s_SERVICE_HOST", strings.ToUpper(rs.Name))
	envSentinelPort := fmt.Sprintf("REDIS_SENTINEL_%s_SERVICE_PORT_SENTINEL", strings.ToUpper(rs.Name))
	shutdownContent := fmt.Sprintf(`#!/usr/bin/env sh
master=""
response_code=""
while [ "$master" = "" ]; do
	echo "Asking sentinel who is master..."
	master=$(redis-cli -h ${%s} -p ${%s} --csv SENTINEL get-master-addr-by-name mymaster | tr ',' ' ' | tr -d '\"' |cut -d' ' -f1)
	sleep 1
done
echo "Master is $master, doing redis save..."
redis-cli SAVE
if [ $master = $(hostname -i) ]; then
	while [ ! "$response_code" = "OK" ]; do
  		response_code=$(redis-cli -h ${%s} -p ${%s} SENTINEL failover mymaster)
		echo "after failover with code $response_code"
		sleep 1
	done
fi`, envSentinelHost, envSentinelPort, envSentinelHost, envSentinelPort)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          labels,
			OwnerReferences: ownerRefs,
		},
		Data: map[string]string{
			"shutdown.sh": shutdownContent,
		},
	}
}

func generateSentinelReadinessProbeConfigMap(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) *corev1.ConfigMap {
	name := util.GetSentinelReadinessCm(rs)
	namespace := rs.Namespace

	labels = util.MergeLabels(labels, generateSelectorLabels(util.RedisRoleName, rs.Name))
	checkContent := `#!/usr/bin/env sh
set -eou pipefail
redis-cli -h $(hostname) -p 26379 ping
slaves=$(redis-cli -h $(hostname) -p 26379 info sentinel|grep master0| grep -Eo 'slaves=[0-9]+' | awk -F= '{print $2}')
status=$(redis-cli -h $(hostname) -p 26379 info sentinel|grep master0| grep -Eo 'status=\w+' | awk -F= '{print $2}')
if [ "$status" != "ok" ]; then 
    exit 1
fi
if [ $slaves -le 1 ]; then
	exit 1
fi
`

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          labels,
			OwnerReferences: ownerRefs,
		},
		Data: map[string]string{
			"readiness.sh": checkContent,
		},
	}
}

func generateRedisStatefulSet(rs *rsv1.RedisSentinel, labels map[string]string,
	ownerRefs []metav1.OwnerReference) *appsv1.StatefulSet {
	name := util.GetRedisName(rs)
	namespace := rs.Namespace

	spec := rs.Spec
	redisCommand := getRedisCommand(rs)
	labels = util.MergeLabels(labels, generateSelectorLabels(util.RedisRoleName, rs.Name))
	volumeMounts := getRedisVolumeMounts(rs)
	volumes := getRedisVolumes(rs)

	probeArg := "redis-cli -h $(hostname)"
	if spec.Password != "" {
		probeArg = fmt.Sprintf("%s -a '%s' ping", probeArg, spec.Password)
	} else {
		probeArg = fmt.Sprintf("%s ping", probeArg)
	}

	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          labels,
			OwnerReferences: ownerRefs,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: name,
			Replicas:    &spec.Size,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: "RollingUpdate",
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: rs.Spec.Annotations,
				},
				Spec: corev1.PodSpec{
					Affinity:         getAffinity(rs.Spec.Affinity, labels),
					Tolerations:      rs.Spec.ToleRations,
					NodeSelector:     rs.Spec.NodeSelector,
					SecurityContext:  getSecurityContext(rs.Spec.SecurityContext),
					ImagePullSecrets: rs.Spec.ImagePullSecrets,
					Containers: []corev1.Container{
						{
							Name:            "redis",
							Image:           rs.Spec.Image,
							ImagePullPolicy: pullPolicy(rs.Spec.ImagePullPolicy),
							Ports: []corev1.ContainerPort{
								{
									Name:          "redis",
									ContainerPort: 6379,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: volumeMounts,
							Command:      redisCommand,
							ReadinessProbe: &corev1.Probe{
								InitialDelaySeconds: graceTime,
								TimeoutSeconds:      5,
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"sh",
											"-c",
											probeArg,
										},
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								InitialDelaySeconds: graceTime,
								TimeoutSeconds:      5,
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"sh",
											"-c",
											probeArg,
										},
									},
								},
							},
							Resources: rs.Spec.Resources,
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{"/bin/sh", "/redis-shutdown/shutdown.sh"},
									},
								},
							},
						},
					},
					Volumes: volumes,
				},
			},
		},
	}

	if rs.Spec.Storage.PersistentVolumeClaim != nil {
		if !rs.Spec.Storage.KeepAfterDeletion {
			// Set an owner reference so the persistent volumes are deleted when the rc is
			rs.Spec.Storage.PersistentVolumeClaim.OwnerReferences = ownerRefs
		}
		ss.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
			*rs.Spec.Storage.PersistentVolumeClaim,
		}
	}

	if rs.Spec.Exporter.Enabled {
		exporter := createRedisExporterContainer(rs)
		ss.Spec.Template.Spec.Containers = append(ss.Spec.Template.Spec.Containers, exporter)
	}

	return ss
}

func generateSentinelStatefulSet(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) *appsv1.StatefulSet {
	name := util.GetSentinelName(rs)
	configMapName := util.GetSentinelName(rs)
	namespace := rs.Namespace

	spec := rs.Spec
	sentinelCommand := getSentinelCommand(rs)
	labels = util.MergeLabels(labels, generateSelectorLabels(util.SentinelRoleName, rs.Name))

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          labels,
			OwnerReferences: ownerRefs,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: util.GetSentinelHeadlessSvc(rs),
			Replicas:    &spec.Sentinel.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: rs.Spec.Sentinel.Annotations,
				},
				Spec: corev1.PodSpec{
					Affinity:         getAffinity(rs.Spec.Sentinel.Affinity, labels),
					Tolerations:      rs.Spec.Sentinel.ToleRations,
					NodeSelector:     rs.Spec.Sentinel.NodeSelector,
					SecurityContext:  getSecurityContext(rs.Spec.Sentinel.SecurityContext),
					ImagePullSecrets: rs.Spec.Sentinel.ImagePullSecrets,
					InitContainers: []corev1.Container{
						{
							Name:            "sentinel-config-copy",
							Image:           rs.Spec.Sentinel.Image,
							ImagePullPolicy: pullPolicy(rs.Spec.Sentinel.ImagePullPolicy),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "sentinel-config",
									MountPath: "/redis",
								},
								{
									Name:      "sentinel-config-writable",
									MountPath: "/redis-writable",
								},
							},
							Command: []string{
								"cp",
								fmt.Sprintf("/redis/%s", util.SentinelConfigFileName),
								fmt.Sprintf("/redis-writable/%s", util.SentinelConfigFileName),
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("32Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("32Mi"),
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "sentinel",
							Image:           rs.Spec.Sentinel.Image,
							ImagePullPolicy: pullPolicy(rs.Spec.Sentinel.ImagePullPolicy),
							Ports: []corev1.ContainerPort{
								{
									Name:          "sentinel",
									ContainerPort: 26379,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "readiness-probe",
									MountPath: "/redis-probe",
								},
								{
									Name:      "sentinel-config-writable",
									MountPath: "/redis",
								},
							},
							Command: sentinelCommand,
							ReadinessProbe: &corev1.Probe{
								InitialDelaySeconds: graceTime,
								PeriodSeconds:       15,
								FailureThreshold:    5,
								TimeoutSeconds:      5,
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"sh",
											"/redis-probe/readiness.sh",
										},
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								InitialDelaySeconds: graceTime,
								TimeoutSeconds:      5,
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"sh",
											"-c",
											"redis-cli -h $(hostname) -p 26379 ping",
										},
									},
								},
							},
							Resources: rs.Spec.Sentinel.Resources,
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "sentinel-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapName,
									},
								},
							},
						},
						{
							Name: "readiness-probe",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: util.GetSentinelReadinessCm(rs),
									},
								},
							},
						},
						{
							Name: "sentinel-config-writable",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}
}

func generatePodDisruptionBudget(name string, namespace string, labels map[string]string, ownerRefs []metav1.OwnerReference, minAvailable intstr.IntOrString) *policyv1beta1.PodDisruptionBudget {
	return &policyv1beta1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          labels,
			OwnerReferences: ownerRefs,
		},
		Spec: policyv1beta1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
		},
	}
}

func generateResourceList(cpu string, memory string) corev1.ResourceList {
	resources := corev1.ResourceList{}
	if cpu != "" {
		resources[corev1.ResourceCPU], _ = resource.ParseQuantity(cpu)
	}
	if memory != "" {
		resources[corev1.ResourceMemory], _ = resource.ParseQuantity(memory)
	}
	return resources
}

func createRedisExporterContainer(rs *rsv1.RedisSentinel) corev1.Container {
	container := corev1.Container{
		Name:            exporterContainerName,
		Image:           rs.Spec.Exporter.Image,
		ImagePullPolicy: pullPolicy(rs.Spec.Exporter.ImagePullPolicy),
		Env: []corev1.EnvVar{
			{
				Name: "REDIS_ALIAS",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          exporterPortName,
				ContainerPort: exporterPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(exporterDefaultLimitCPU),
				corev1.ResourceMemory: resource.MustParse(exporterDefaultLimitMemory),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(exporterDefaultRequestCPU),
				corev1.ResourceMemory: resource.MustParse(exporterDefaultRequestMemory),
			},
		},
	}
	if rs.Spec.Password != "" {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  redisPasswordEnv,
			Value: rs.Spec.Password,
		})
	}
	return container
}

func createPodAntiAffinity(hard bool, labels map[string]string) *corev1.PodAntiAffinity {
	if hard {
		// Return a HARD anti-affinity (no same pods on one node)
		return &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					TopologyKey: util.HostnameTopologyKey,
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: labels,
					},
				},
			},
		}
	}

	// Return a SOFT anti-affinity
	return &corev1.PodAntiAffinity{
		PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
			{
				Weight: 100,
				PodAffinityTerm: corev1.PodAffinityTerm{
					TopologyKey: util.HostnameTopologyKey,
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: labels,
					},
				},
			},
		},
	}
}

func getSecurityContext(secctx *corev1.PodSecurityContext) *corev1.PodSecurityContext {
	if secctx != nil {
		return secctx
	}

	return nil
}

func getQuorum(rs *rsv1.RedisSentinel) int32 {
	return rs.Spec.Sentinel.Replicas/2 + 1
}

func getRedisVolumeMounts(rs *rsv1.RedisSentinel) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		//{
		//	Name:      redisConfigurationVolumeName,
		//	MountPath: "/redis",
		//},
		{
			Name:      redisShutdownConfigurationVolumeName,
			MountPath: "/redis-shutdown",
		},
		{
			Name:      getRedisDataVolumeName(rs),
			MountPath: "/data",
		},
	}

	return volumeMounts
}

func getRedisVolumes(rs *rsv1.RedisSentinel) []corev1.Volume {
	shutdownConfigMapName := util.GetRedisShutdownConfigMapName(rs)

	executeMode := int32(0744)
	volumes := []corev1.Volume{
		{
			Name: redisShutdownConfigurationVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: shutdownConfigMapName,
					},
					DefaultMode: &executeMode,
				},
			},
		},
	}

	dataVolume := getRedisDataVolume(rs)
	if dataVolume != nil {
		volumes = append(volumes, *dataVolume)
	}

	return volumes
}

func getRedisDataVolume(rs *rsv1.RedisSentinel) *corev1.Volume {
	// This will find the volumed desired by the user. If no volume defined
	// an EmptyDir will be used by default
	switch {
	case rs.Spec.Storage.PersistentVolumeClaim != nil:
		return nil
	case rs.Spec.Storage.EmptyDir != nil:
		return &corev1.Volume{
			Name: redisStorageVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: rs.Spec.Storage.EmptyDir,
			},
		}
	default:
		return &corev1.Volume{
			Name: redisStorageVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
	}
}

func getRedisDataVolumeName(rs *rsv1.RedisSentinel) string {
	switch {
	case rs.Spec.Storage.PersistentVolumeClaim != nil:
		return rs.Spec.Storage.PersistentVolumeClaim.Name
	case rs.Spec.Storage.EmptyDir != nil:
		return redisStorageVolumeName
	default:
		return redisStorageVolumeName
	}
}

func getRedisCommand(rs *rsv1.RedisSentinel) []string {
	if len(rs.Spec.Command) > 0 {
		return rs.Spec.Command
	}

	cmds := []string{
		"redis-server",
		"--slaveof 127.0.0.1 6379",
		"--tcp-keepalive 60",
		"--save 900 1",
		"--save 300 10",
	}

	if rs.Spec.Password != "" {
		cmds = append(cmds, fmt.Sprintf("--requirepass '%s'", rs.Spec.Password),
			fmt.Sprintf("--masterauth '%s'", rs.Spec.Password))
	}

	return cmds
}

func getSentinelCommand(rs *rsv1.RedisSentinel) []string {
	if len(rs.Spec.Sentinel.Command) > 0 {
		return rs.Spec.Sentinel.Command
	}
	return []string{
		"redis-server",
		fmt.Sprintf("/redis/%s", util.SentinelConfigFileName),
		"--sentinel",
	}
}

func getAffinity(affinity *corev1.Affinity, labels map[string]string) *corev1.Affinity {
	if affinity != nil {
		return affinity
	}

	// Return a SOFT anti-affinity
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey: util.HostnameTopologyKey,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
					},
				},
			},
		},
	}
}

// newHeadLessSvcForCR creates a new headless service for the given Cluster.
func newHeadLessSvcForCR(rs *rsv1.RedisSentinel, labels map[string]string, ownerRefs []metav1.OwnerReference) *corev1.Service {
	sentinelPort := corev1.ServicePort{Name: "sentinel", Port: 26379}
	labels = util.MergeLabels(labels, generateSelectorLabels(util.SentinelRoleName, rs.Name))
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          labels,
			Name:            util.GetSentinelHeadlessSvc(rs),
			Namespace:       rs.Namespace,
			OwnerReferences: ownerRefs,
		},
		Spec: corev1.ServiceSpec{
			Ports:     []corev1.ServicePort{sentinelPort},
			Selector:  labels,
			ClusterIP: corev1.ClusterIPNone,
		},
	}

	return svc
}

func pullPolicy(specPolicy corev1.PullPolicy) corev1.PullPolicy {
	if specPolicy == "" {
		return corev1.PullAlways
	}
	return specPolicy
}
