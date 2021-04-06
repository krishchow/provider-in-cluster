package redis

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/crossplane-contrib/provider-in-cluster/apis/cache/v1alpha1"
)

var _ Client = &redisClient{}

// Client is the interface for the redis client
type Client interface {
	CreateOrUpdate(ctx context.Context, obj client.Object) (controllerutil.OperationResult, error)
	ParseInputSecret(ctx context.Context, redis v1alpha1.Redis) (string, error)

	DeleteRedisDeployment(ctx context.Context, redis *v1alpha1.Redis) error
	DeleteRedisService(ctx context.Context, redis *v1alpha1.Redis) error
}

type redisClient struct {
	kube client.Client
}

func (r *redisClient) CreateOrUpdate(ctx context.Context, obj client.Object) (controllerutil.OperationResult, error) {
	return controllerutil.CreateOrUpdate(ctx, r.kube, obj, func() error {
		return nil
	})
}

func (r *redisClient) ParseInputSecret(ctx context.Context, redis v1alpha1.Redis) (string, error) {
	panic("implement me")
}

func (r *redisClient) DeleteRedisDeployment(ctx context.Context, redis *v1alpha1.Redis) error {
	panic("implement me")
}

func (r *redisClient) DeleteRedisService(ctx context.Context, redis *v1alpha1.Redis) error {
	panic("implement me")
}

// MakeRedisDeployment creates the Deployment for a Redis instancee
func MakeRedisDeployment(ps *v1alpha1.Redis) *appsv1.Deployment {
	return nil
}

// MakeDefaultRedisPodContainers creates the containers for the Redis pod
func MakeDefaultRedisPodContainers(ps *v1alpha1.Redis) []v1.Container {
	return nil
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
	return nil
}
