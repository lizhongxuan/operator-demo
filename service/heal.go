package service


import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"github.com/go-logr/logr"
	rsv1 "redis-sentinel/api/v1"
	"redis-sentinel/pkg/k8s"
	"redis-sentinel/controllers/redisclient"
	"redis-sentinel/pkg/util"
)

// RedisClusterHeal defines the intercace able to fix the problems on the redis clusters
type RedisClusterHeal interface {
	MakeMaster(ip string, auth *util.AuthConfig) error
	SetOldestAsMaster(rs *rsv1.RedisSentinel, auth *util.AuthConfig) error
	SetMasterOnAll(masterIP string, rs *rsv1.RedisSentinel, auth *util.AuthConfig) error
	NewSentinelMonitor(ip string, monitor string, rs *rsv1.RedisSentinel, auth *util.AuthConfig) error
	RestoreSentinel(ip string, auth *util.AuthConfig) error
	SetSentinelCustomConfig(ip string, rs *rsv1.RedisSentinel, auth *util.AuthConfig) error
	SetRedisCustomConfig(ip string, rs *rsv1.RedisSentinel, auth *util.AuthConfig) error
}

// RedisClusterHealer is our implementation of RedisClusterCheck intercace
type RedisClusterHealer struct {
	k8sService  k8s.Services
	redisClient redisclient.Client
	logger      logr.Logger
}

// NewRedisClusterHealer creates an object of the RedisClusterChecker struct
func NewRedisClusterHealer(k8sService k8s.Services, redisClient redisclient.Client, logger logr.Logger) *RedisClusterHealer {
	return &RedisClusterHealer{
		k8sService:  k8sService,
		redisClient: redisClient,
		logger:      logger,
	}
}

func (r *RedisClusterHealer) MakeMaster(ip string, auth *util.AuthConfig) error {
	return r.redisClient.MakeMaster(ip, auth)
}

// SetOldestAsMaster puts all redis to the same master, choosen by order of appearance
func (r *RedisClusterHealer) SetOldestAsMaster(rs *rsv1.RedisSentinel, auth *util.AuthConfig) error {
	ssp, err := r.k8sService.GetStatefulSetPods(rs.Namespace, util.GetRedisName(rs))
	if err != nil {
		return err
	}
	if len(ssp.Items) < 1 {
		return errors.New("number of redis pods are 0")
	}

	// Order the pods so we start by the oldest one
	sort.Slice(ssp.Items, func(i, j int) bool {
		return ssp.Items[i].CreationTimestamp.Before(&ssp.Items[j].CreationTimestamp)
	})

	newMasterIP := ""
	for _, pod := range ssp.Items {
		if newMasterIP == "" {
			newMasterIP = pod.Status.PodIP
			r.logger.V(2).Info(fmt.Sprintf("new master is %s with ip %s", pod.Name, newMasterIP))
			if err := r.redisClient.MakeMaster(newMasterIP, auth); err != nil {
				return err
			}
		} else {
			r.logger.V(2).Info(fmt.Sprintf("making pod %s slave of %s", pod.Name, newMasterIP))
			if err := r.redisClient.MakeSlaveOf(pod.Status.PodIP, newMasterIP, auth); err != nil {
				return err
			}
		}
	}
	return nil
}

// SetMasterOnAll puts all redis nodes as a slave of a given master
func (r *RedisClusterHealer) SetMasterOnAll(masterIP string, rs *rsv1.RedisSentinel, auth *util.AuthConfig) error {
	ssp, err := r.k8sService.GetStatefulSetPods(rs.Namespace, util.GetRedisName(rs))
	if err != nil {
		return err
	}
	for _, pod := range ssp.Items {
		if pod.Status.PodIP == masterIP {
			r.logger.V(2).Info(fmt.Sprintf("ensure pod %s is master", pod.Name))
			if err := r.redisClient.MakeMaster(masterIP, auth); err != nil {
				return err
			}
		} else {
			r.logger.V(2).Info(fmt.Sprintf("making pod %s slave of %s", pod.Name, masterIP))
			if err := r.redisClient.MakeSlaveOf(pod.Status.PodIP, masterIP, auth); err != nil {
				return err
			}
		}
	}
	return nil
}

// NewSentinelMonitor changes the master that Sentinel has to monitor
func (r *RedisClusterHealer) NewSentinelMonitor(ip string, monitor string, rs *rsv1.RedisSentinel, auth *util.AuthConfig) error {
	r.logger.V(2).Info("sentinel is not monitoring the correct master, changing...")
	quorum := strconv.Itoa(int(getQuorum(rs)))
	return r.redisClient.MonitorRedis(ip, monitor, quorum, auth)
}

// RestoreSentinel clear the number of sentinels on memory
func (r *RedisClusterHealer) RestoreSentinel(ip string, auth *util.AuthConfig) error {
	r.logger.V(2).Info(fmt.Sprintf("restoring sentinel %s...", ip))
	return r.redisClient.ResetSentinel(ip, auth)
}

// SetSentinelCustomConfig will call sentinel to set the configuration given in config
func (r *RedisClusterHealer) SetSentinelCustomConfig(ip string, rs *rsv1.RedisSentinel, auth *util.AuthConfig) error {
	if len(rs.Spec.Sentinel.CustomConfig) == 0 {
		return nil
	}
	r.logger.V(2).Info(fmt.Sprintf(fmt.Sprintf("setting the custom config on sentinel %s: %v", ip, rs.Spec.Sentinel.CustomConfig)))
	return r.redisClient.SetCustomSentinelConfig(ip, rs.Spec.Sentinel.CustomConfig, auth)
}

// SetRedisCustomConfig will call redis to set the configuration given in config
func (r *RedisClusterHealer) SetRedisCustomConfig(ip string, rs *rsv1.RedisSentinel, auth *util.AuthConfig) error {
	if len(rs.Spec.Config) == 0 && len(auth.Password) == 0 {
		return nil
	}

	//if len(auth.Password) != 0 {
	//	rc.Spec.Config["requirepass"] = auth.Password
	//	rc.Spec.Config["masterauth"] = auth.Password
	//}

	r.logger.V(2).Info(fmt.Sprintf("setting the custom config on redis %s: %v", ip, rs.Spec.Config))

	return r.redisClient.SetCustomRedisConfig(ip, rs.Spec.Config, auth)
}

