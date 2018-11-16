package util

import (
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/openshift/console-operator/pkg/controller"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
)

func SharedLabels() map[string]string {
	return map[string]string{
		"app": controller.OpenShiftConsoleName,
	}
}

func LabelsForConsole() map[string]string {
	baseLabels := SharedLabels()

	extraLabels := map[string]string{
		"component": "ui",
	}
	// we want to deduplicate, so doing these two loops.
	allLabels := map[string]string{}

	for key, value := range baseLabels {
		allLabels[key] = value
	}
	for key, value := range extraLabels {
		allLabels[key] = value
	}
	return allLabels
}

func SharedMeta() v1.ObjectMeta {
	return v1.ObjectMeta{
		Name:      controller.OpenShiftConsoleName,
		Namespace: controller.OpenShiftConsoleName,
		Labels:    SharedLabels(),
	}
}

func LogYaml(obj runtime.Object) {
	// REALLY NOISY, but handy for debugging:
	// deployJSON, err := json.Marshal(d)
	deployYAML, err := yaml.Marshal(obj)
	if err != nil {
		logrus.Info("failed to show deployment yaml in log")
	}
	logrus.Infof("Deploying: %v", string(deployYAML))
}

// objects can have more than one ownerRef, potentially
func AddOwnerRef(obj v1.Object, ownerRef *v1.OwnerReference) {
	if obj != nil {
		if ownerRef != nil {
			obj.SetOwnerReferences(append(obj.GetOwnerReferences(), *ownerRef))
		}
	}
}

func OwnerRefFrom(cr *v1alpha1.Console) *v1.OwnerReference {
	if cr != nil {
		truthy := true
		return &v1.OwnerReference{
			APIVersion: cr.APIVersion,
			Kind:       cr.Kind,
			Name:       cr.Name,
			UID:        cr.UID,
			Controller: &truthy,
		}
	}
	return nil
}

// borrowed from library-go
// https://github.com/openshift/library-go/blob/master/pkg/operator/v1alpha1helpers/helpers.go
func GetImageEnv() string {
	return os.Getenv("IMAGE")
}
