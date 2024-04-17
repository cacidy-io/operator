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

	cacidyiov1alpha1 "cacidy.io/runner/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

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
	runner *cacidyiov1alpha1.Runner
}

//+kubebuilder:rbac:groups=cacidy.io,resources=runners,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cacidy.io,resources=runners/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cacidy.io,resources=runners/finalizers,verbs=update

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
	log := log.FromContext(ctx)

	runner := &cacidyiov1alpha1.Runner{}
	if err := r.Get(ctx, req.NamespacedName, runner); apierrors.IsNotFound(err) {
		log.Info(fmt.Sprintf("Runner '%s' was not found. The object may have been deleted", req.Name))
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.addFinalizers(ctx, runner); err != nil {
		log.Error(err, "failed adding runner finalizers")
		return ctrl.Result{}, err
	}

	if requeue, err := r.syncAppChecksum(ctx, runner); err != nil {
		log.Error(err, "failed syncing the application repository checksum")
		return ctrl.Result{}, err
	} else if requeue {
		return ctrl.Result{}, nil
	}

	if err := r.deployEngine(req.Name, req.Namespace, runner); err != nil {
		log.Error(err, "failed deploying engine")
		return ctrl.Result{}, err
	}

	if err := r.resolveFinalizers(ctx, req, runner); err != nil {
		log.Error(err, "failed resolving finalizers")
		return ctrl.Result{Requeue: true}, nil
	}

	if requeue, err := r.runPipeline(ctx, req, runner); err != nil {
		log.Error(err, "failed running pipeline")
		return ctrl.Result{}, err
	} else if requeue {
		return ctrl.Result{}, nil
	}

	if requeue, err := r.checkPipelineJobStatus(ctx, req, runner); err != nil {
		log.Error(err, "failed checking the pipeline status")
		return ctrl.Result{}, err
	} else if requeue {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: time.Second * appCheckumInterval}, nil
}

func (r *RunnerReconciler) addFinalizers(ctx context.Context, runner *cacidyiov1alpha1.Runner) error {
	if !controllerutil.ContainsFinalizer(runner, engineFinalizer) {
		if ok := controllerutil.AddFinalizer(runner, engineFinalizer); !ok {
			return errors.New("failed adding engine finalizer")
		}
		if err := r.Update(ctx, runner); err != nil {
			return fmt.Errorf("%s failed to update runner while adding finalizer", err.Error())
		}
	}
	return nil
}

func (r *RunnerReconciler) resolveFinalizers(ctx context.Context, req ctrl.Request, runner *cacidyiov1alpha1.Runner) error {
	if runner.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(runner, engineFinalizer) {
			if err := r.destroyEngine(req.Name, req.Namespace, runner); err != nil {
				return err
			}
			if ok := controllerutil.RemoveFinalizer(runner, engineFinalizer); !ok {
				return errors.New("failed removing finalizer")
			}
			if err := r.Update(ctx, runner); err != nil {
				return err
			}
			return nil

		}
	}
	return nil
}

func (r *RunnerReconciler) syncAppChecksum(ctx context.Context, runner *cacidyiov1alpha1.Runner) (bool, error) {
	app := newApplication(&runner.Spec.Application)
	appAuth, err := gitAuth()
	if err != nil {
		return false, err
	}
	checksum, err := app.getChecksum(appAuth)
	if err != nil {
		return false, err
	}
	if runner.Status.Checksum == checksum {
		return false, nil
	} else if runner.Status.Checksum == "" {
		runner.Status.State = cacidyiov1alpha1.Ready
	} else if runner.Status.Checksum != checksum {
		runner.Status.State = cacidyiov1alpha1.OutOfSync
	}
	runner.Status.Checksum = checksum
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.Status().Update(ctx, runner)
	})
	if err != nil {
		return true, nil
	}
	return true, nil
}

func (r *RunnerReconciler) deployEngine(name, namespace string, runner *cacidyiov1alpha1.Runner) error {
	eng := newEngine(name, namespace, runner.Spec.Engine)
	return eng.deploy(r.Client, context.Background())
}

func (r *RunnerReconciler) destroyEngine(name, namespace string, runner *cacidyiov1alpha1.Runner) error {
	eng := newEngine(name, namespace, runner.Spec.Engine)
	return eng.destroy(r.Client, context.Background())
}

func (r *RunnerReconciler) runPipeline(ctx context.Context, req ctrl.Request, runner *cacidyiov1alpha1.Runner) (bool, error) {
	if runner.Status.State != cacidyiov1alpha1.OutOfSync {
		return false, nil
	}
	eng := newEngine(req.Name, req.Namespace, runner.Spec.Engine)
	podName, err := eng.podName(r.Client, context.Background())
	if err != nil {
		return false, err
	}
	job := pipelineJob{
		Name:          req.Name,
		Namespace:     req.Namespace,
		EnginePodName: podName,
		Checksum:      runner.Status.Checksum,
		appSpec:       runner.Spec.Application,
		moduleSpec:    runner.Spec.Module,
	}
	runner.Status.State = cacidyiov1alpha1.Running
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.Status().Update(ctx, runner)
	})
	if err != nil {
		return true, nil
	}
	return true, job.run(r.Client, context.Background())
}

func (r *RunnerReconciler) checkPipelineJobStatus(ctx context.Context, req ctrl.Request, runner *cacidyiov1alpha1.Runner) (bool, error) {
	if runner.Status.State != cacidyiov1alpha1.Running {
		return false, nil
	}
	job := pipelineJob{
		Name:      req.Name,
		Namespace: req.Namespace,
		Checksum:  runner.Status.Checksum,
	}
	status, err := job.status(r.Client, ctx)
	if err != nil {
		return false, err
	}
	if status == pendingJobStatus {
		return false, nil
	}
	if status == failedJobStatus {
		runner.Status.State = cacidyiov1alpha1.Failed
	}
	if status == completeJobStatus {
		runner.Status.State = cacidyiov1alpha1.Complete
	}
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.Status().Update(ctx, runner)
	})
	if err != nil {
		return true, nil
	}
	return true, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RunnerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cacidyiov1alpha1.Runner{}).
		Complete(r)
}
