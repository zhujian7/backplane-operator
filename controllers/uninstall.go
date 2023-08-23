// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"

	backplanev1 "github.com/stolostron/backplane-operator/api/v1"
	"github.com/stolostron/backplane-operator/pkg/toggle"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	// The uninstallList is the list of all resources from previous installs to remove. Items can be removed
	// from this list in future releases if they are sure to not exist prior to the current installer version
	uninstallList = func(backplaneConfig *backplanev1.MultiClusterEngine) []*unstructured.Unstructured {
		removals := []*unstructured.Unstructured{
			newUnstructured(
				types.NamespacedName{Name: "hypershift-deployment", Namespace: backplaneConfig.Spec.TargetNamespace},
				schema.GroupVersionKind{Group: "", Kind: "ServiceAccount", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "hypershift-deployment-controller", Namespace: backplaneConfig.Spec.TargetNamespace},
				schema.GroupVersionKind{Group: "apps", Kind: "Deployment", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "open-cluster-management:hypershift-preview:hypershiftDeployment-leader-election", Namespace: backplaneConfig.Spec.TargetNamespace},
				schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Kind: "Role", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "open-cluster-management:hypershift-preview:hypershiftDeployment-leader-election", Namespace: backplaneConfig.Spec.TargetNamespace},
				schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Kind: "RoleBinding", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "open-cluster-management:hypershift-preview:hypershift-deployment-controller"},
				schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "open-cluster-management:hypershift-preview:hypershift-deployment-controller"},
				schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRoleBinding", Version: "v1"},
			),
			// managed-serviceaccount
			newUnstructured(
				types.NamespacedName{Name: "managed-serviceaccount", Namespace: backplaneConfig.Spec.TargetNamespace},
				schema.GroupVersionKind{Group: "", Kind: "ServiceAccount", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "managed-serviceaccount-addon-manager", Namespace: backplaneConfig.Spec.TargetNamespace},
				schema.GroupVersionKind{Group: "apps", Kind: "Deployment", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "open-cluster-management:managed-serviceaccount:managed-serviceaccount"},
				schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "open-cluster-management:managed-serviceaccount:managed-serviceaccount"},
				schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRoleBinding", Version: "v1"},
			),
		}

		return removals
	}
)

func newUnstructured(nn types.NamespacedName, gvk schema.GroupVersionKind) *unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName(nn.Name)
	u.SetNamespace((nn.Namespace))
	return &u
}

// ensureRemovalsGone validates successful removal of everything in the uninstallList. Return on first error encounter.
func (r *MultiClusterEngineReconciler) ensureRemovalsGone(backplaneConfig *backplanev1.MultiClusterEngine) (ctrl.Result, error) {
	removals := uninstallList(backplaneConfig)

	namespacedName := types.NamespacedName{Name: "hypershift-removals", Namespace: backplaneConfig.Spec.TargetNamespace}
	r.StatusManager.AddComponent(toggle.DisabledStatus(namespacedName, removals))

	allResourcesDeleted := true
	for i := range removals {
		gone, err := r.uninstall(backplaneConfig, removals[i])
		if err != nil {
			return ctrl.Result{}, err
		}
		if !gone {
			allResourcesDeleted = false
		}
	}

	if !allResourcesDeleted {
		return ctrl.Result{RequeueAfter: requeuePeriod}, nil
	}

	return ctrl.Result{}, nil
}

// uninstall return true if resource does not exist and returns an error if a GET or DELETE errors unexpectedly. A false response without error
// means the resource is in the process of deleting.
func (r *MultiClusterEngineReconciler) uninstall(backplaneConfig *backplanev1.MultiClusterEngine, u *unstructured.Unstructured) (bool, error) {

	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      u.GetName(),
		Namespace: u.GetNamespace(),
	}, u)

	if errors.IsNotFound(err) {
		return true, nil
	}

	// Get resource. Successful if it doesn't exist.
	if err != nil {
		// Error that isn't due to the resource not existing
		return false, err
	}

	// If resource has deletionTimestamp then re-reconcile and don't try deleting
	if u.GetDeletionTimestamp() != nil {
		return false, nil
	}

	// Attempt deleting resource. No error does not necessarily mean the resource is gone.
	err = r.Client.Delete(context.TODO(), u)
	if err != nil {
		return false, err
	}
	return false, nil
}
