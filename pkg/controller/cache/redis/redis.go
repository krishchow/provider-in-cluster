/*
Copyright 2020 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane-contrib/provider-in-cluster/apis/cache/v1alpha1"
	clients "github.com/crossplane-contrib/provider-in-cluster/pkg/client"
	"github.com/crossplane-contrib/provider-in-cluster/pkg/client/cache/redis"
)

const (
	errUnexpectedObject = "the managed resource is not a redis resource"
	errDelete           = "failed to delete the Redis resource"
	errDeploymentMsg    = "failed to get redis deployment"
	errServiceMsg       = "failed to get redis service"
	errDeployCreateMsg  = "failed to create or update redis deployment"
	errSVCCreateMsg     = "failed to create or update redis service"
)

// SetupRedis adds a controller that reconciles Redis instances.
func SetupRedis(mgr ctrl.Manager, l logging.Logger, rl workqueue.RateLimiter) error {
	name := managed.ControllerName(v1alpha1.RedisGroupKind)
	redisLogger := l.WithValues("controller", name)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(controller.Options{
			RateLimiter: rl,
		}).
		For(&v1alpha1.Redis{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1alpha1.RedisGroupVersionKind),
			managed.WithExternalConnecter(&connector{kube: mgr.GetClient(), newClientFn: redis.NewClient, logger: redisLogger}),
			managed.WithReferenceResolver(managed.NewAPISimpleReferenceResolver(mgr.GetClient())),
			managed.WithLogger(redisLogger),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
}

type connector struct {
	kube        client.Client
	newClientFn func(kube client.Client) redis.Client
	logger      logging.Logger
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.Redis)
	if !ok {
		return nil, errors.New(errUnexpectedObject)
	}

	c.logger.Debug("Connecting")

	rc, err := clients.GetProviderConfigRC(ctx, cr, c.kube)
	if err != nil {
		return nil, err
	}

	kube, err := client.New(rc, client.Options{})
	if err != nil {
		return nil, err
	}

	return &external{redisClient: redis.NewClient(kube), kube: kube, logger: c.logger}, nil
}

type external struct {
	kube        client.Client
	redisClient redis.Client
	logger      logging.Logger
}

func (e *external) Observe(ctx context.Context, mgd resource.Managed) (managed.ExternalObservation, error) {
	ps, ok := mgd.(*v1alpha1.Redis)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errUnexpectedObject)
	}

	e.logger.Debug("observe called", "resource", ps)

	initializeDefaults(ps)

	// check deployment status
	dpl := &appsv1.Deployment{}
	err := e.kube.Get(ctx, types.NamespacedName{Name: ps.Name, Namespace: ps.Namespace}, dpl)
	if err != nil {
		e.logger.Debug(errDeploymentMsg, "err", err)
		return managed.ExternalObservation{}, errors.Wrap(resource.IgnoreNotFound(err), errDeploymentMsg)
	}

	// check if deployment is ready and return connection details
	dplAvailable := false
	for _, s := range dpl.Status.Conditions {
		if s.Type == appsv1.DeploymentAvailable && s.Status == v1.ConditionTrue {
			dplAvailable = true
			break
		}
	}

	// deployment is in progress
	if !dplAvailable {
		e.logger.Debug("deployment currently not available")
		return managed.ExternalObservation{ResourceExists: true}, nil
	}

	svc := &v1.Service{}
	err = e.kube.Get(ctx, types.NamespacedName{Name: ps.Name, Namespace: ps.Namespace}, svc)
	if err != nil {
		e.logger.Debug(errServiceMsg, "err", err)
		return managed.ExternalObservation{ResourceExists: true}, errors.Wrap(err, errServiceMsg)
	}

	ip := svc.Spec.ClusterIP
	e.logger.Debug("redis service", "ip", fmt.Sprintf("%+v", ip))

	ps.SetConditions(runtimev1alpha1.Available())

	return managed.ExternalObservation{ConnectionDetails: map[string][]byte{
		runtimev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(ip),
	}, ResourceExists: true, ResourceUpToDate: true}, nil
}

func (e *external) Create(ctx context.Context, mgd resource.Managed) (managed.ExternalCreation, error) {
	ps, ok := mgd.(*v1alpha1.Redis)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errUnexpectedObject)
	}

	e.logger.Debug("create called", "resource", ps)

	// deploy credentials secret
	password := e.redisClient.ParseInputSecret(ctx, *ps)
	conn := make(map[string][]byte)
	if password != nil {
		conn[runtimev1alpha1.ResourceCredentialsSecretPasswordKey] = []byte(*password)
	}

	// deploy deployment
	if _, err := e.redisClient.CreateOrUpdate(ctx, redis.MakeRedisDeployment(ps)); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errDeployCreateMsg)
	}
	// deploy service
	if _, err := e.redisClient.CreateOrUpdate(ctx, redis.MakeDefaultRedisService(ps)); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errSVCCreateMsg)
	}

	conn[runtimev1alpha1.ResourceCredentialsSecretPortKey] = []byte(strconv.Itoa(redis.DefaultRedisPort))

	return managed.ExternalCreation{
		ConnectionDetails: conn,
	}, nil
}

func (e *external) Update(ctx context.Context, mgd resource.Managed) (managed.ExternalUpdate, error) {
	ps, ok := mgd.(*v1alpha1.Redis)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errUnexpectedObject)
	}

	e.logger.Debug("update called", "resource", ps)

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mgd resource.Managed) error {
	ps, ok := mgd.(*v1alpha1.Redis)
	if !ok {
		return errors.New(errUnexpectedObject)
	}

	e.logger.Debug("delete called", "resource", ps)
	err := e.redisClient.DeleteRedisDeployment(ctx, ps)
	if err != nil {
		return errors.Wrap(err, errDelete)
	}
	err = e.redisClient.DeleteRedisService(ctx, ps)
	if err != nil {
		return errors.Wrap(err, errDelete)
	}

	return nil
}

func initializeDefaults(pg *v1alpha1.Redis) {
	// We need to set the default namespace here for the PV/PVC.
	if strings.TrimSpace(pg.Namespace) == "" {
		pg.Namespace = "default"
	}
}
