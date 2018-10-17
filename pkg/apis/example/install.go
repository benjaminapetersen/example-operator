package kubeapiserver

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	examplev1alpha1 "github.com/enj/example-operator/pkg/apis/example/v1alpha1"
)

const (
	GroupName = "exampleoperator.crd.monis.app"
)

var (
	schemeBuilder = runtime.NewSchemeBuilder(examplev1alpha1.Install)
	// Install is a function which adds every version of this group to a scheme
	Install = schemeBuilder.AddToScheme
)

func Resource(resource string) schema.GroupResource {
	return schema.GroupResource{Group: GroupName, Resource: resource}
}

func Kind(kind string) schema.GroupKind {
	return schema.GroupKind{Group: GroupName, Kind: kind}
}
