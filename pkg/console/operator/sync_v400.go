package operator

import (
	v1alpha12 "github.com/openshift/api/operator/v1alpha1"
	"github.com/openshift/console-operator/pkg/console/subresource/secret"
	"github.com/openshift/console-operator/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	// kube
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	errutil "k8s.io/apimachinery/pkg/util/errors"
	// openshift
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	// operator
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	servicesub "github.com/openshift/console-operator/pkg/console/subresource/service"
	configmapsub "github.com/openshift/console-operator/pkg/console/subresource/configmap"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
	oauthsub "github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"

)

// runs the standard v4.0.0 reconcile loop
func sync_v400(co *ConsoleOperator, console *v1alpha1.Console) error {
	// aggregate the errors from this:
	allErrors := []error{}
	toUpdate := false


	// apply service
	_, svcChanged, err := resourceapply.ApplyService(co.serviceClient, servicesub.DefaultService(console))
	if err != nil {
		allErrors = append(allErrors, err)
	}
	toUpdate = toUpdate || svcChanged


	// apply route
	rt, rtChanged, err := routesub.ApplyRoute(co.routeClient, routesub.DefaultRoute(console))

	if err != nil {
		allErrors = append(allErrors, err)
	}
	toUpdate = toUpdate || rtChanged


	// apply configmap (needs route)
	_, cmChanged, err := resourceapply.ApplyConfigMap(co.configMapClient, configmapsub.DefaultConfigMap(console, rt))
	if err != nil {
		allErrors = append(allErrors, err)
	}
	toUpdate = toUpdate || cmChanged


	// apply deployment is a bit more involved as it needs information about version & if we should
	// force a rollout of the pods.  at this point, configMap updates are the bool for this
	defaultDeployment := deploymentsub.DefaultDeployment(console)
	versionAvailability := &v1alpha12.VersionAvailability{
		Version: console.Spec.Version,
	}
	deploymentGeneration := resourcemerge.ExpectedDeploymentGeneration(defaultDeployment, versionAvailability)
	redeployPods := cmChanged
	_, dpChanged, err := resourceapply.ApplyDeployment(co.deploymentClient, defaultDeployment, deploymentGeneration, redeployPods)
	if err != nil {
		allErrors = append(allErrors, err)
	}
	toUpdate = toUpdate || dpChanged


	// shared
	sharedOAuthSecretBits := crypto.Random256BitsString()


	// apply oauth (needs route)
	defaultOauthClient := oauthsub.RegisterConsoleToOAuthClient(oauthsub.DefaultOauthClient(), rt, sharedOAuthSecretBits)
	_, oauthChanged, err := oauthsub.ApplyOAuth(co.oauthClient, defaultOauthClient)
	if err != nil {
		allErrors = append(allErrors, err)
	}
	toUpdate = toUpdate || oauthChanged


	// apply secret
	_, secretChanged, err := resourceapply.ApplySecret(co.secretsClient, secret.DefaultSecret(console,sharedOAuthSecretBits))
	if err != nil {
		allErrors = append(allErrors, err)
	}
	toUpdate = toUpdate || secretChanged


	// if any of our resources have svcChanged, we should update the CR. otherwise, skip this step.
	if toUpdate {
		// update status on CR
		// pass in each of the above resources, possibly the
		// boolean for "changed" as well, and then assign a status
		// on the CR.Status.  Be sure to account for a nil return value
		// as some of our resources (oauthlient, configmap) may simply
		// not be possible to create if they lack the appropriate inputs.
		// in this case, the Status should CLEARLY indicate this to the user.
		// Once the resource is correctly created, the status should be updated
		// again.  This should be pretty simple and straightforward work.
		// update cluster operator status... i believe this
		// should be automatic so long as the CR.Status is
		// properly filled out with the appropriate values.
	}

	return errutil.FilterOut(errutil.NewAggregate(allErrors), apierrors.IsNotFound)
}


//func DeleteAllResources(cr *v1alpha1.Console) error {
//	var errs []error
//	for _, fn := range []func(*v1alpha1.Console) error{
//		DeleteService,
//		DeleteRoute,
//		DeleteConfigMap,
//		DeleteDeployment,
//		DeleteOAuthSecret,
//		// we don't own it and can't create or delete it. however, we can update it
//		NeutralizeOAuthClient,
//	} {
//		errs = append(errs, fn(cr))
//	}
//	return errutil.FilterOut(errutil.NewAggregate(errs), errors.IsNotFound)
//}
