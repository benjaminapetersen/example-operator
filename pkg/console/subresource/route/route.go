package route

import (
	routev1 "github.com/openshift/api/route/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	"github.com/openshift/console-operator/pkg/controller"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// TODO: ApplyRoute
// - should look like resourceapply.ApplyService
// - should be PR'd to client-go
func ApplyRoute(client routeclient.RoutesGetter, required *routev1.Route) (*routev1.Route, bool, error) {
	// first, get or create
	existing, err := client.Routes(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.Routes(required.Namespace).Create(required)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}
	// otherwise update
	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)

	toWrite := existing
	toWrite.Spec = *required.Spec.DeepCopy()

	actual, err := client.Routes(required.Namespace).Update(toWrite)
	return actual, true, err
}

func DefaultRoute(cr *v1alpha1.Console) *routev1.Route {
	meta := util.SharedMeta()
	meta.Name = controller.OpenShiftConsoleShortName
	weight := int32(100)
	route := &routev1.Route{
		ObjectMeta: meta,
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: meta.Name,
				Weight: &weight,
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString("https"),
			},
			TLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationReencrypt,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
			WildcardPolicy: routev1.WildcardPolicyNone,
		},
	}
	util.AddOwnerRef(route, util.OwnerRefFrom(cr))
	return route
}
