package controller

import (
	"context"
	"fmt"
	"log"

	cacidyiov1alpha1 "cacidy.io/runner/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	pipelineContainerImage = "cacidy/runner:v0.1.0"
	pipelineJobRole        = "cacidy-pipeline-job"
)

type pipelineJob struct {
	Name          string
	Namespace     string
	EnginePodName string
	Checksum      string
	appSpec       cacidyiov1alpha1.RunnerSpecApplication
	moduleSpec    cacidyiov1alpha1.RunnerSpecPipelineModule
}

func (job *pipelineJob) name() string {
	return fmt.Sprintf("%s-%s", job.Name, job.Checksum)
}

func (job *pipelineJob) serviceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipelineJobRole,
			Namespace: job.Namespace,
		},
	}
}

func (job *pipelineJob) role() *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipelineJobRole,
			Namespace: job.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods/exec"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get"},
			},
		},
	}
}

func (job *pipelineJob) rolebinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipelineJobRole,
			Namespace: job.Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      pipelineJobRole,
				Namespace: job.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     pipelineJobRole,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}

func (job *pipelineJob) jobEnv() []corev1.EnvVar {
	env := []corev1.EnvVar{
		{
			Name:  "APP_URL",
			Value: job.appSpec.Repository,
		},
		{
			Name:  "APP_BRANCH",
			Value: job.appSpec.Branch,
		},
		{
			Name:  "MODULE_URL",
			Value: job.moduleSpec.Repository,
		},
		{
			Name:  "MODULE_REVISION",
			Value: job.moduleSpec.Revision,
		},
		{
			Name:  "MODULE_FUNCTION",
			Value: job.moduleSpec.Function,
		},
		{
			Name:  "DAGGER_ENGINE_NAMESPACE",
			Value: job.Namespace,
		},
		{
			Name:  "DAGGER_ENGINE_POD_NAME",
			Value: job.EnginePodName,
		},
	}
	return env
}

func (job *pipelineJob) jobEnvFrom() []corev1.EnvFromSource {
	envFrom := []corev1.EnvFromSource{}
	if job.moduleSpec.Args != "" {
		envFrom = append(envFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: job.moduleSpec.CloudTokenSecret,
				},
			},
		})
	}
	if job.moduleSpec.CloudTokenSecret != "" {
		envFrom = append(envFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: fmt.Sprintf(job.moduleSpec.CloudTokenSecret),
				},
			},
		})
	}
	return envFrom
}

func (job *pipelineJob) job() *batchv1.Job {
	backoffLimit := int32(0)
	completions := int32(1)
	terminationGracePeriod := int64(300)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      job.name(),
			Namespace: job.Namespace,
			Annotations: map[string]string{
				"checksum.app.cacidy.io": job.Checksum,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Completions:  &completions,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							ImagePullPolicy: "Always",
							Name:            "pipeline",
							Image:           pipelineContainerImage,
							Env:             job.jobEnv(),
							EnvFrom:         job.jobEnvFrom(),
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu":    resource.MustParse("15m"),
									"memory": resource.MustParse("64Mi"),
								},
								Limits: corev1.ResourceList{
									"memory": resource.MustParse("256Mi"),
								},
							},
						},
					},
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					ServiceAccountName:            pipelineJobRole,
				},
			},
		},
	}
}

func (job *pipelineJob) getObjects() []client.Object {
	objs := []client.Object{
		job.serviceAccount(),
		job.role(),
		job.rolebinding(),
	}
	return objs
}

func (job *pipelineJob) run(cli client.Client, ctx context.Context) error {
	for _, resource := range job.getObjects() {
		if err := cli.Update(ctx, resource); err != nil {
			if apierrors.IsInvalid(err) {
				log.Println(err)
			}
			if apierrors.IsNotFound(err) {
				if err := cli.Create(ctx, resource); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}
	if err := cli.Create(ctx, job.job()); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

type jobStatus string

const (
	pendingJobStatus  jobStatus = "Pending"
	failedJobStatus   jobStatus = "Failed"
	completeJobStatus jobStatus = "Complete"
)

func (job *pipelineJob) status(cli client.Client, ctx context.Context) (jobStatus, error) {
	status := pendingJobStatus
	obj := &batchv1.Job{}
	if err := cli.Get(ctx, types.NamespacedName{
		Namespace: job.Namespace,
		Name:      job.name(),
	}, obj); err != nil {
		return status, err
	}
	if obj.Status.Failed > 0 {
		status = failedJobStatus
	}
	if obj.Status.Succeeded > 0 {
		status = completeJobStatus
	}
	return status, nil
}
