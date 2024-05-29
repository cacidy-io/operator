/*
Copyright 2024.

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

package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	cacidyiov1alpha1 "github.com/cacidy-io/operator/api/v1alpha1"
	"github.com/cacidy-io/operator/internal/controller/pipeline"
	"github.com/cacidy-io/operator/internal/controller/runner"
	"github.com/go-git/go-git/v5/plumbing/transport"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// runnerRequeueInterval .
const runnerRequeueInterval = 10

type statusReason string

const (
	InitializingStatusReason statusReason = "Initializing"
	RunningStatusReason      statusReason = "Running"
	IdleStatusReason         statusReason = "Idle"
	FinalizingStatusReason   statusReason = "Finalizing"
)

// RunnerReconciler reconciles a Runner object
type RunnerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cacidy.io,resources=runners,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cacidy.io,resources=runners/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cacidy.io,resources=runners/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=create;update;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=create;update;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=create;update;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Runner object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *RunnerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rnr := &cacidyiov1alpha1.Runner{}
	if err := r.Get(ctx, req.NamespacedName, rnr); apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	if rnr.GetDeletionTimestamp() != nil {
		if done, err := r.resolveFinalizers(ctx, req, rnr); err != nil {
			return ctrl.Result{Requeue: true}, nil
		} else if done {
			return ctrl.Result{}, nil
		}
	}

	if err := r.addFinalizers(ctx, rnr); err != nil {
		return ctrl.Result{}, err
	}

	if requeue, err := r.syncAppChecksum(ctx, rnr); err != nil {
		return ctrl.Result{}, err
	} else if requeue {
		return ctrl.Result{}, nil
	}

	if err := r.deployEngine(req.Name, req.Namespace, rnr); err != nil {
		return ctrl.Result{}, err
	}

	if requeue, err := r.createPipelineJob(ctx, req, rnr); err != nil {
		return ctrl.Result{}, err
	} else if requeue {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: time.Second * runnerRequeueInterval}, nil
}

// addFinalizers adds the deletion finalizers to the runner.
func (r *RunnerReconciler) addFinalizers(ctx context.Context, rnr *cacidyiov1alpha1.Runner) error {
	if !controllerutil.ContainsFinalizer(rnr, runner.EngineFinalizer) {
		if ok := controllerutil.AddFinalizer(rnr, runner.EngineFinalizer); !ok {
			return errors.New("failed adding engine finalizer")
		}
		if err := r.Update(ctx, rnr); err != nil {
			return fmt.Errorf("%s failed to update runner while adding finalizer", err.Error())
		}
	}
	return nil
}

// resolveFinalizers resolves active finalizers before deletion.
func (r *RunnerReconciler) resolveFinalizers(ctx context.Context, req ctrl.Request, rnr *cacidyiov1alpha1.Runner) (bool, error) {
	done := false
	if controllerutil.ContainsFinalizer(rnr, runner.EngineFinalizer) {
		if err := r.destroyEngine(req.Name, req.Namespace, rnr); err != nil {
			return false, err
		}
		if ok := controllerutil.RemoveFinalizer(rnr, runner.EngineFinalizer); !ok {
			return false, errors.New("failed removing finalizer")
		}
		if err := r.Update(ctx, rnr); err != nil {
			return false, err
		}
	}
	done = true
	return done, nil
}

// syncAppChecksum gets the latest commit hash from the repository.
func (r *RunnerReconciler) syncAppChecksum(ctx context.Context, rnr *cacidyiov1alpha1.Runner) (bool, error) {
	var appAuth transport.AuthMethod
	requeue := false
	app := runner.NewApplication(&rnr.Spec.Application)
	// authSecret, err := util.GetAuthSecret(r.Client, ctx, rnr.Spec.Application.SecretStore, rnr.Namespace)
	// if err != nil {
	// 	return requeue, err
	// }
	// if authSecret != nil {
	// 	appAuth = util.GitAuth(authSecret)
	// }
	checksum, err := app.GetChecksum(appAuth)
	if err != nil {
		return requeue, err
	}
	if rnr.Status.Checksum == checksum {
		return requeue, nil
	}
	requeue = true
	if rnr.Status.State == "" {
		rnr.Status.State = cacidyiov1alpha1.RunnerSynced
	} else {
		rnr.Status.State = cacidyiov1alpha1.RunnerOutOfSync
	}
	rnr.Status.Checksum = checksum
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.Status().Update(ctx, rnr)
	})
	if err != nil {
		return requeue, err
	}
	return requeue, nil
}

// deployEngine installs an instance of the engine.
func (r *RunnerReconciler) deployEngine(name, namespace string, rnr *cacidyiov1alpha1.Runner) error {
	eng := runner.New(name, namespace, rnr.Spec.Engine)
	return eng.Deploy(r.Client, context.Background())
}

// destroyEngine deletes an installed instance of the engine.
func (r *RunnerReconciler) destroyEngine(name, namespace string, rnr *cacidyiov1alpha1.Runner) error {
	eng := runner.New(name, namespace, rnr.Spec.Engine)
	return eng.Destroy(r.Client, context.Background())
}

// createPipelineJob creates a new pipeline job resource.
func (r *RunnerReconciler) createPipelineJob(ctx context.Context, req ctrl.Request, rnr *cacidyiov1alpha1.Runner) (bool, error) {
	requeue := false
	if rnr.Status.State != cacidyiov1alpha1.RunnerOutOfSync {
		return requeue, nil
	}
	job := pipeline.NewJob(pipeline.JobOptions{
		AppName:             req.Name,
		Namespace:           req.Namespace,
		Revision:            rnr.Status.Checksum,
		Module:              rnr.Spec.Module,
		Application:         rnr.Spec.Application,
		RetentionPeriodDays: rnr.Spec.RetentionPeriodDays,
	})
	if err := r.Create(ctx, job); err != nil {
		return requeue, err
	}
	rnr.Status.State = cacidyiov1alpha1.RunnerSynced
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.Status().Update(ctx, rnr)
	})
	if err != nil {
		return requeue, err
	}
	return requeue, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RunnerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cacidyiov1alpha1.Runner{}).
		Complete(r)
}
