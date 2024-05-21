package pipeline

import (
	"context"
	"fmt"
	"log"

	cacidyiov1alpha1 "github.com/cacidy-io/operator/api/v1alpha1"
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

type Pipeline struct {
	AppName         string
	Namespace       string
	EnginePodName   string
	Revision        string
	Application     cacidyiov1alpha1.Application
	Module          cacidyiov1alpha1.PipelineModule
	RetentionPeriod int
}

func (pipe *Pipeline) name() string {
	return fmt.Sprintf("%s-%s", pipe.AppName, pipe.Revision[:7])
}

func (pipe *Pipeline) serviceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipelineJobRole,
			Namespace: pipe.Namespace,
		},
	}
}

func (pipe *Pipeline) role() *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipelineJobRole,
			Namespace: pipe.Namespace,
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

func (pipe *Pipeline) rolebinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipelineJobRole,
			Namespace: pipe.Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      pipelineJobRole,
				Namespace: pipe.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     pipelineJobRole,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}

func (pipe *Pipeline) jobEnv() []corev1.EnvVar {
	env := []corev1.EnvVar{
		{
			Name:  "CACIDY_APPLICATION_URL",
			Value: pipe.Application.Repository,
		},
		{
			Name:  "CACIDY_APPLICATION_REVISION",
			Value: pipe.Revision,
		},
		{
			Name:  "CACIDY_MODULE_URL",
			Value: pipe.Module.Repository,
		},
		{
			Name:  "CACIDY_MODULE_REVISION",
			Value: pipe.Module.Revision,
		},
		{
			Name:  "CACIDY_CALL_FUNCTION",
			Value: pipe.Module.Function,
		},
		{
			Name:  "CACIDY_CALL_SOURCE_AS",
			Value: pipe.Module.SourceAs,
		},
		{
			Name:  "CACIDY_DAGGER_ENGINE_POD_NAME",
			Value: pipe.EnginePodName,
		},
		{
			Name:  "CACIDY_DAGGER_ENGINE_NAMESPACE",
			Value: pipe.Namespace,
		},
	}
	if pipe.Application.AuthSecret != "" {
		env = append(env, []corev1.EnvVar{
			{
				Name: "CACIDY_APPLICATION_USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pipe.Application.AuthSecret,
						},
						Key: "username",
					},
				},
			},
			{
				Name: "CACIDY_APPLICATION_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pipe.Application.AuthSecret,
						},
						Key: "password",
					},
				},
			},
		}...)
	}
	if pipe.Module.AuthSecret != "" {
		env = append(env, []corev1.EnvVar{
			{
				Name: "CACIDY_MODULE_USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pipe.Module.AuthSecret,
						},
						Key: "username",
					},
				},
			},
			{
				Name: "CACIDY_MODULE_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pipe.Module.AuthSecret,
						},
						Key: "password",
					},
				},
			},
		}...)
	}
	return env
}

func (pipe *Pipeline) jobEnvFrom() []corev1.EnvFromSource {
	envFrom := []corev1.EnvFromSource{}
	if pipe.Module.CloudTokenSecret != "" {
		envFrom = append(envFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: pipe.Module.CloudTokenSecret,
				},
			},
		})
	}
	return envFrom
}

func (pipe *Pipeline) job() *batchv1.Job {
	backoffLimit := int32(0)
	completions := int32(1)
	terminationGracePeriodSeconds := int64(300)
	ttlSecondsAfterFinished := int32(pipe.RetentionPeriod * 86400)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipe.name(),
			Namespace: pipe.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			Completions:             &completions,
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							ImagePullPolicy: "Always",
							Name:            "pipeline",
							Image:           pipelineContainerImage,
							Env:             pipe.jobEnv(),
							EnvFrom:         pipe.jobEnvFrom(),
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
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					ServiceAccountName:            pipelineJobRole,
				},
			},
		},
	}
}

func (pipe *Pipeline) getObjects() []client.Object {
	objs := []client.Object{
		pipe.serviceAccount(),
		pipe.role(),
		pipe.rolebinding(),
	}
	return objs
}

func (pipe *Pipeline) Run(cli client.Client, ctx context.Context) error {
	for _, resource := range pipe.getObjects() {
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
	job := pipe.job()
	if err := cli.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

type jobStatus string

const (
	PendingJobStatus  jobStatus = "Pending"
	FailedJobStatus   jobStatus = "Failed"
	CompleteJobStatus jobStatus = "Complete"
)

func (pipe *Pipeline) Status(cli client.Client, ctx context.Context) (jobStatus, error) {
	status := PendingJobStatus
	obj := &batchv1.Job{}
	if err := cli.Get(ctx, types.NamespacedName{
		Namespace: pipe.Namespace,
		Name:      pipe.name(),
	}, obj); err != nil {
		return status, err
	}
	if obj.Status.Failed > 0 {
		status = FailedJobStatus
	}
	if obj.Status.Succeeded > 0 {
		status = CompleteJobStatus
	}
	return status, nil
}

type JobOptions struct {
	AppName             string
	Namespace           string
	Revision            string
	Module              cacidyiov1alpha1.PipelineModule
	Application         cacidyiov1alpha1.Application
	RetentionPeriodDays int
}

func NewJob(options JobOptions) *cacidyiov1alpha1.PipelineJob {
	name := fmt.Sprintf("%s-%s", options.AppName, options.Revision[:7])
	return &cacidyiov1alpha1.PipelineJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: options.Namespace,
			Labels: map[string]string{
				"app": options.AppName,
			},
		},
		Spec: cacidyiov1alpha1.PipelineJobSpec{
			Module:              options.Module,
			Application:         options.Application,
			Revision:            options.Revision,
			RetentionPeriodDays: options.RetentionPeriodDays,
		},
	}
}
