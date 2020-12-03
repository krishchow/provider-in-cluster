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

package operator

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	operaterv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	"github.com/crossplane-contrib/provider-in-cluster/apis/operator/v1alpha1"
	"github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ Client = &operatorClient{}

type Client interface {
	CreateOperator(ctx context.Context, obj *v1alpha1.Operator) error
	GetPackageManifest(ctx context.Context, obj *v1alpha1.Operator) (*operators.PackageManifest, error)
	ParsePackageManifest(ctx context.Context, op *v1alpha1.Operator, obj *operators.PackageManifest) (csv *string, single bool, multi bool, all bool)
	CheckCSV(ctx context.Context, csv string, op *v1alpha1.Operator) (bool, bool)
	DeleteCSV (ctx context.Context, csv string, op *v1alpha1.Operator) error
}

type operatorClient struct {
	kube client.Client
	logger logging.Logger
}

func NewClient(kube client.Client, logger logging.Logger) Client {
	return operatorClient{kube: kube, logger: logger}
}

func (o operatorClient) CreateOperator(ctx context.Context, op *v1alpha1.Operator) error {
	sub := operaterv1alpha1.Subscription{
		Spec: &operaterv1alpha1.SubscriptionSpec{
			CatalogSource:          op.Spec.ForProvider.CatalogSource,
			CatalogSourceNamespace: op.Spec.ForProvider.CatalogSourceNamespace,
			Package:                op.Spec.ForProvider.OperatorName,
			Channel:                op.Spec.ForProvider.Channel,
		},
	}
	sub.Namespace = op.Namespace
	sub.Name = op.Name
	return o.kube.Create(ctx, &sub)
}

func (o operatorClient) GetPackageManifest(ctx context.Context, obj *v1alpha1.Operator) (*operators.PackageManifest, error) {
	pkg := operators.PackageManifest{}
	err := o.kube.Get(ctx, client.ObjectKey{
		Namespace: obj.Spec.ForProvider.CatalogSourceNamespace,
		Name:      obj.Spec.ForProvider.OperatorName,
	} ,&pkg)
	if err != nil {
		return nil, err
	}
	return &pkg, nil
}

func (o operatorClient) ParsePackageManifest(ctx context.Context, op *v1alpha1.Operator, obj *operators.PackageManifest) (csv *string, single bool, multi bool, all bool) {
	var channel *operators.PackageChannel = nil
	for _, v := range obj.Status.Channels {
		if v.Name == op.Spec.ForProvider.OperatorName {
			channel = &v
			break
		}
	}
	if channel == nil {
		return
	}
	csv = &channel.CurrentCSV
	for _, v := range channel.CurrentCSVDesc.InstallModes {
		if v.Type == operaterv1alpha1.InstallModeTypeSingleNamespace {
			single = true
		} else if v.Type == operaterv1alpha1.InstallModeTypeMultiNamespace {
			multi = true
		} else if v.Type == operaterv1alpha1.InstallModeTypeAllNamespaces {
			all = true
		}
	}

	return
}

func (o operatorClient) CheckCSV(ctx context.Context, csv string, op *v1alpha1.Operator) (bool, bool) {
	cluster := operaterv1alpha1.ClusterServiceVersion{}
	err := o.kube.Get(ctx, client.ObjectKey{
		Namespace: op.Namespace,
		Name:      csv,
	}, &cluster)
	if err != nil {
		return false, false
	}
	return true, cluster.Status.Phase == operaterv1alpha1.CSVPhaseSucceeded
}

func (o operatorClient) DeleteCSV (ctx context.Context, csv string, op *v1alpha1.Operator) error {
	cluster := operaterv1alpha1.ClusterServiceVersion{}
	err := o.kube.Get(ctx, client.ObjectKey{
		Namespace: op.Namespace,
		Name:      csv,
	}, &cluster)
	if err != nil {
		return err
	}
	return o.kube.Delete(ctx, &cluster)
}

func (o operatorClient) DeleteSubscription (ctx context.Context, csv string, op *v1alpha1.Operator) error {
	cluster := operaterv1alpha1.Subscription{}
	err := o.kube.Get(ctx, client.ObjectKey{
		Namespace: op.Namespace,
		Name:      op.Name,
	}, &cluster)
	if err != nil {
		return err
	}
	return o.kube.Delete(ctx, &cluster)
}