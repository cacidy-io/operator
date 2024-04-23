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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cacidyiov1alpha1 "cacidy.io/runner/api/v1alpha1"
	"cacidy.io/runner/internal/controller/pipeline"
	"cacidy.io/runner/internal/controller/runner"
)

const pipelineJobRequeueInterval = 3

// PipelineJobReconciler reconciles a PipelineJob object
type PipelineJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cacidy.io,resources=pipelinejobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cacidy.io,resources=pipelinejobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cacidy.io,resources=pipelinejobs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PipelineJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *PipelineJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	job := &cacidyiov1alpha1.PipelineJob{}
	if err := r.Get(ctx, req.NamespacedName, job); apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	if requeue, err := r.runPipeline(ctx, req, job); err != nil {
		return ctrl.Result{}, err
	} else if requeue {
		return ctrl.Result{}, nil
	}

	if requeue, err := r.checkPipelineJobStatus(ctx, req, job); err != nil {
		return ctrl.Result{}, err
	} else if requeue {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: pipelineJobRequeueInterval}, nil
}

// runPipeline .
func (r *PipelineJobReconciler) runPipeline(ctx context.Context, req ctrl.Request, job *cacidyiov1alpha1.PipelineJob) (bool, error) {
	if job.Status.State == "" {
		job.Status.State = cacidyiov1alpha1.PipelineStarting
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			return r.Status().Update(ctx, job)
		}); err != nil {
			return false, err
		}
		return true, nil
	}
	if job.Status.State != cacidyiov1alpha1.PipelineStarting {
		return false, nil
	}
	eng := runner.New(job.ObjectMeta.Labels["app"], req.Namespace)
	podName, err := eng.PodName(r.Client, context.Background())
	if err != nil {
		return false, err
	}
	pipe := pipeline.Pipeline{
		AppName:         job.ObjectMeta.Labels["app"],
		Namespace:       req.Namespace,
		Revision:        job.Spec.Revision,
		Application:     job.Spec.Application,
		Module:          job.Spec.Module,
		RetentionPeriod: job.Spec.RetentionPeriodDays,
		EnginePodName:   podName,
	}
	if err := pipe.Run(r.Client, context.Background()); err != nil {
		return false, err
	}
	job.Status.State = cacidyiov1alpha1.PipelineRunning
	if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.Status().Update(ctx, job)
	}); err != nil {
		return true, nil
	}
	return true, nil
}

// checkPipelineJobStatus .
func (r *PipelineJobReconciler) checkPipelineJobStatus(ctx context.Context, req ctrl.Request, job *cacidyiov1alpha1.PipelineJob) (bool, error) {
	if job.Status.State != cacidyiov1alpha1.PipelineRunning {
		return false, nil
	}
	pipe := pipeline.Pipeline{
		AppName:   job.ObjectMeta.Labels["app"],
		Namespace: req.Namespace,
		Revision:  job.Spec.Revision,
	}
	status, err := pipe.Status(r.Client, ctx)
	if err != nil {
		return false, err
	}
	if status == pipeline.PendingJobStatus {
		return false, nil
	}
	if status == pipeline.FailedJobStatus {
		job.Status.State = cacidyiov1alpha1.PipelineFailed
	}
	if status == pipeline.CompleteJobStatus {
		job.Status.State = cacidyiov1alpha1.PipelineComplete
	}
	if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.Status().Update(ctx, job)
	}); err != nil {
		return true, nil
	}
	return true, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PipelineJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cacidyiov1alpha1.PipelineJob{}).
		Complete(r)
}
