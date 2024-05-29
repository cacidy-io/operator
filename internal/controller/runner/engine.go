package runner

import (
	"context"
	"errors"
	"fmt"

	cacidyiov1alpha1 "github.com/cacidy-io/operator/api/v1alpha1"
	"github.com/pelletier/go-toml/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	engineVersion    = "v0.11.4"
	engineContainer  = "registry.dagger.io/engine"
	EngineFinalizer  = "finalizer.engine.cacidy.io"
	engineConfigPath = "/etc/dagger/engine.toml"
	buildkitBasePath = "/etc/buildkit"
	magicacheURL     = "https://api.dagger.cloud/magicache"
	cpuRequests      = "0.015"
	memoryRequests   = "256Mi"
)

type engine struct {
	Name      string
	Namespace string
	Settings  cacidyiov1alpha1.RunnerEngineSettings
}

type buildkitConfig struct {
	Debug        bool     `toml:"debug"`
	Trace        bool     `toml:"trace"`
	Entitlements []string `toml:"insecure-entitlements"`
}

func (eng *engine) metaName() string {
	return fmt.Sprintf("%s-dagger-engine", eng.Name)
}

func (eng *engine) engineConfigMap() (*corev1.ConfigMap, error) {
	config := &buildkitConfig{
		Debug: eng.Settings.Debug,
		Trace: true,
	}
	if eng.Settings.InsecureRootCapabilities {
		config.Entitlements = []string{
			"security.insecure",
		}
	}
	configToml, err := toml.Marshal(config)
	if err != nil {
		return nil, err
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      eng.metaName(),
			Namespace: eng.Namespace,
		},
		Data: map[string]string{
			"engine.toml": string(configToml),
		},
	}, nil
}

func (eng *engine) deploymentEnv() ([]corev1.EnvVar, []corev1.EnvFromSource) {
	env := []corev1.EnvVar{}
	if eng.Settings.MagicacheSecret != "" {
		env = append(env, corev1.EnvVar{
			Name:  "_EXPERIMENTAL_DAGGER_CACHESERVICE_URL",
			Value: magicacheURL,
		})
	}
	envFrom := []corev1.EnvFromSource{}
	if eng.Settings.MagicacheSecret != "" {
		envFrom = []corev1.EnvFromSource{
			{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: eng.Settings.MagicacheSecret,
					},
				},
			},
		}
	}
	return env, envFrom
}

func (eng *engine) deploymentVolumes() ([]corev1.VolumeMount, []corev1.Volume) {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "dagger-engine-config",
			MountPath: engineConfigPath,
			SubPath:   "engine.toml",
			ReadOnly:  true,
		},
	}
	volumes := []corev1.Volume{
		{
			Name: "dagger-engine-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: eng.metaName(),
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "engine.toml",
							Path: "engine.toml",
						},
					},
				},
			},
		},
	}
	if eng.Settings.StorageEnabled {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "dagger-volume",
			MountPath: "/var/lib/dagger",
		})
		volumes = append(volumes, corev1.Volume{
			Name: "dagger-volume",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: eng.metaName(),
				},
			},
		})
	}
	return volumeMounts, volumes
}

func (eng *engine) deploymentResources() (*corev1.ResourceRequirements, error) {
	memory, err := resource.ParseQuantity(eng.Settings.Memory)
	if err != nil {
		return nil, fmt.Errorf("%s %s", "failed parsing memory limit:", err.Error())
	}
	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse(cpuRequests),
			"memory": resource.MustParse(memoryRequests),
		},
		Limits: corev1.ResourceList{
			"memory": memory,
		},
	}, nil
}

func (eng *engine) deployment() (*appsv1.Deployment, error) {
	privileged := true
	terminationGracePeriod := int64(300)
	env, envFrom := eng.deploymentEnv()
	volumeMounts, volumes := eng.deploymentVolumes()
	resources, err := eng.deploymentResources()
	if err != nil {
		return nil, err
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      eng.metaName(),
			Namespace: eng.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": eng.metaName(),
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: "Recreate",
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: eng.metaName(),
					Labels: map[string]string{
						"app": eng.metaName(),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "dagger-engine",
							Image: fmt.Sprintf("%s:%s", engineContainer, engineVersion),
							Args: []string{
								"--oci-max-parallelism",
								"num-cpu",
							},
							Env:     env,
							EnvFrom: envFrom,
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"ALL",
									},
								},
							},
							Resources: *resources,
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"buildctl",
											"debug",
											"workers",
										},
									},
								},
							},
							VolumeMounts: volumeMounts,
						},
					},
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					Volumes:                       volumes,
				},
			},
		},
	}
	return deployment, nil
}

func (eng *engine) dataPVC() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      eng.metaName(),
			Namespace: eng.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse(eng.Settings.CacheSize),
				},
			},
		},
	}
}

func (eng *engine) getObjects() ([]client.Object, error) {
	config, err := eng.engineConfigMap()
	if err != nil {
		return nil, err
	}
	deployment, err := eng.deployment()
	if err != nil {
		return nil, err
	}
	return []client.Object{
		config,
		eng.dataPVC(),
		deployment,
	}, nil
}

func (eng *engine) Deploy(cli client.Client, ctx context.Context) error {
	resources, err := eng.getObjects()
	if err != nil {
		return err
	}
	for _, resource := range resources {
		if err := cli.Update(ctx, resource); err != nil && !apierrors.IsInvalid(err) {
			if apierrors.IsNotFound(err) {
				if err := cli.Create(ctx, resource); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}
	return nil
}

func (eng *engine) Destroy(cli client.Client, ctx context.Context) error {
	resources, err := eng.getObjects()
	if err != nil {
		return err
	}
	for _, resource := range resources {
		if err := cli.Delete(ctx, resource); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (eng *engine) PodName(cli client.Client, ctx context.Context) (string, error) {
	list := &corev1.PodList{}
	if err := cli.List(ctx, list, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"app": eng.metaName(),
		}),
	}); err != nil {
		return "", fmt.Errorf("%s failed getting the engine deployment", err.Error())
	}
	if len(list.Items) < 1 {
		return "", errors.New("no engine pod could be found for pipeline execution")
	}
	return list.Items[0].Name, nil
}

func New(name, namespace string, settings ...cacidyiov1alpha1.RunnerEngineSettings) *engine {
	eng := &engine{Name: name, Namespace: namespace}
	if len(settings) > 0 {
		eng.Settings = settings[0]
	}
	return eng
}
