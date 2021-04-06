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

	"github.com/pkg/errors"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane-contrib/provider-in-cluster/apis/database/v1alpha1"
)

const (
	errUnexpectedObject    = "the managed resource is not a Postgres resource"
)

// SetupRedis adds a controller that reconciles Redis instances.
func SetupRedis(mgr ctrl.Manager, l logging.Logger, rl workqueue.RateLimiter) error {
	name := managed.ControllerName(v1alpha1.PostgresGroupKind)
	postgresLogger := l.WithValues("controller", name)
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(controller.Options{
			RateLimiter: rl,
		}).
		For(&v1alpha1.Postgres{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1alpha1.PostgresGroupVersionKind),
			managed.WithExternalConnecter(&connector{kube: mgr.GetClient(), logger: postgresLogger}),
			managed.WithReferenceResolver(managed.NewAPISimpleReferenceResolver(mgr.GetClient())),
			managed.WithLogger(postgresLogger),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
}

type connector struct {
	kube        client.Client
	logger      logging.Logger
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	_, ok := mg.(*v1alpha1.Postgres)
	if !ok {
		return nil, errors.New(errUnexpectedObject)
	}

	c.logger.Debug("Connecting")

	return &external{kube: c.kube, logger: c.logger}, nil
}

type external struct {
	kube   client.Client
	logger logging.Logger
}

func (e *external) Observe(ctx context.Context, mgd resource.Managed) (managed.ExternalObservation, error) {
	ps, ok := mgd.(*v1alpha1.Postgres)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errUnexpectedObject)
	}

	e.logger.Debug("observe called", "resource", ps)

	ps.SetConditions(runtimev1alpha1.Available())

	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
}

func (e *external) Create(ctx context.Context, mgd resource.Managed) (managed.ExternalCreation, error) {
	ps, ok := mgd.(*v1alpha1.Postgres)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errUnexpectedObject)
	}

	e.logger.Debug("create called", "resource", ps)

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mgd resource.Managed) (managed.ExternalUpdate, error) {
	ps, ok := mgd.(*v1alpha1.Postgres)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errUnexpectedObject)
	}

	e.logger.Debug("update called", "resource", ps)

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mgd resource.Managed) error {
	ps, ok := mgd.(*v1alpha1.Postgres)
	if !ok {
		return errors.New(errUnexpectedObject)
	}

	e.logger.Debug("delete called", "resource", ps)

	return nil
}
