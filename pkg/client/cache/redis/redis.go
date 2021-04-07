package redis

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/crossplane-contrib/provider-in-cluster/apis/cache/v1alpha1"
	"github.com/crossplane-contrib/provider-in-cluster/pkg/controller/utils"
)

var _ Client = &redisClient{}

const (
	// DefaultRedisPort is the default port for redis
	DefaultRedisPort = 6379
	// ImageTagRedis is the tag for the default redis image used
	ImageTagRedis = "bitnami/redis:6.0.12"
)

// Client is the interface for the redis client
type Client interface {
	CreateOrUpdate(ctx context.Context, obj client.Object) (controllerutil.OperationResult, error)
	ParseInputSecret(ctx context.Context, redis v1alpha1.Redis) *string

	DeleteRedisDeployment(ctx context.Context, redis *v1alpha1.Redis) error
	DeleteRedisService(ctx context.Context, redis *v1alpha1.Redis) error
}

type redisClient struct {
	kube client.Client
}

// NewClient creates the redis client with interface
func NewClient(kube client.Client) Client {
	return &redisClient{kube: kube}
}

// ParseInputSecret parses the secret to get the database password
func (r *redisClient) ParseInputSecret(ctx context.Context, redis v1alpha1.Redis) *string {
	if redis.Spec.ForProvider.PasswordSecretRef == nil {
		return nil
	}
	nn := types.NamespacedName{
		Name:      redis.Spec.ForProvider.PasswordSecretRef.Name,
		Namespace: redis.Spec.ForProvider.PasswordSecretRef.Namespace,
	}
	s := &v1.Secret{}
	if err := r.kube.Get(ctx, nn, s); err != nil {
		return nil
	}
	pw := string(s.Data[redis.Spec.ForProvider.PasswordSecretRef.Key])
	return &pw
}

func (r *redisClient) CreateOrUpdate(ctx context.Context, obj client.Object) (controllerutil.OperationResult, error) {
	return controllerutil.CreateOrUpdate(ctx, r.kube, obj, func() error {
		return nil
	})
}

func (r *redisClient) DeleteRedisDeployment(ctx context.Context, redis *v1alpha1.Redis) error {
	dpl := appsv1.Deployment{}
	err := r.kube.Get(ctx, client.ObjectKey{
		Name:      redis.Name,
		Namespace: redis.Namespace,
	}, &dpl)
	if err != nil {
		return nil
	}
	return r.kube.Delete(ctx, &dpl)
}

func (r *redisClient) DeleteRedisService(ctx context.Context, redis *v1alpha1.Redis) error {
	svc := v1.Service{}
	err := r.kube.Get(ctx, client.ObjectKey{
		Name:      redis.Name,
		Namespace: redis.Namespace,
	}, &svc)
	if err != nil {
		return nil
	}
	return r.kube.Delete(ctx, &svc)
}

// MakeRedisDeployment creates the Deployment for a Redis instance
func MakeRedisDeployment(ps *v1alpha1.Redis) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ps.Name,
			Namespace: ps.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Replicas: utils.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"deployment": ps.Name,
				},
			},
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: makeDefaultRedisPodContainers(ps),
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"deployment": ps.Name,
					},
				},
			},
		},
	}
}

func getRedisPassword(ps *v1alpha1.Redis) v1.EnvVar {
	mp := ps.Spec.ForProvider.ConfigVariables
	if mp != nil {
		if val, ok := mp["REDIS_PASSWORD"]; ok {
			return envVarFromValue("REDIS_PASSWORD", val)
		}
	}
	if ps.Spec.ForProvider.PasswordSecretRef != nil {
		return v1.EnvVar{
			Name: "REDIS_PASSWORD",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: ps.Spec.ForProvider.PasswordSecretRef.Name,
					},
					Key:      ps.Spec.ForProvider.PasswordSecretRef.Key,
					Optional: utils.Boolean(false),
				},
			},
		}
	}
	return envVarFromValue("ALLOW_EMPTY_PASSWORD", "yes")
}

// makeDefaultRedisPodContainers creates the containers for the Redis pod
func makeDefaultRedisPodContainers(ps *v1alpha1.Redis) []v1.Container {
	envVariables := make([]v1.EnvVar, 0)
	envVariables = append(envVariables, getRedisPassword(ps))
	mp := ps.Spec.ForProvider.ConfigVariables
	if mp != nil {
		delete(mp, "ALLOW_EMPTY_PASSWORD")
		delete(mp, "REDIS_PASSWORD")
		for k, v := range mp {
			envVariables = append(envVariables, envVarFromValue(k, v))
		}
	}

	memory := "2Gi"
	if ps.Spec.ForProvider.MemoryLimit != nil {
		memory = *ps.Spec.ForProvider.MemoryLimit
	}

	return []v1.Container{
		{
			Name:  ps.Name,
			Image: ImageTagRedis,
			Ports: []v1.ContainerPort{
				{
					ContainerPort: DefaultRedisPort,
					Protocol:      v1.ProtocolTCP,
				},
			},
			Env: envVariables,
			Resources: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("250m"),
					v1.ResourceMemory: resource.MustParse(memory),
				},
				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("50m"),
					v1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
			LivenessProbe: &v1.Probe{
				Handler: v1.Handler{
					TCPSocket: &v1.TCPSocketAction{
						Port: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: DefaultRedisPort,
						},
					},
				},
				InitialDelaySeconds: 30,
				PeriodSeconds:       10,
				TimeoutSeconds:      0,
				SuccessThreshold:    0,
				FailureThreshold:    0,
			},
			ImagePullPolicy: v1.PullIfNotPresent,
		},
	}
}

// envVarFromValue creates the environment variable for the pods
func envVarFromValue(envVarName, value string) v1.EnvVar {
	return v1.EnvVar{
		Name:  envVarName,
		Value: value,
	}
}

// MakeDefaultRedisService is responsible for creating the Service for redis
func MakeDefaultRedisService(ps *v1alpha1.Redis) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ps.Name,
			Namespace: ps.Namespace,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:       "redis",
					Protocol:   v1.ProtocolTCP,
					Port:       DefaultRedisPort,
					TargetPort: intstr.FromInt(DefaultRedisPort),
				},
			},
			Selector: map[string]string{"deployment": ps.Name},
		},
	}
}
