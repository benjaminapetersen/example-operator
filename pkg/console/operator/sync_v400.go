package operator

import (
	"github.com/sirupsen/logrus"
	// kube
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	errutil "k8s.io/apimachinery/pkg/util/errors"
	// openshift
	oauthv1 "github.com/openshift/api/oauth/v1"
	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/console/subresource/secret"
	"github.com/openshift/console-operator/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
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
func sync_v400(co *ConsoleOperator, consoleConfig *v1alpha1.Console) error {
	// aggregate the errors from this:
	allErrors := []error{}
	toUpdate := false


	// apply service
	svc, svcChanged, svcErr := resourceapply.ApplyService(co.serviceClient, servicesub.DefaultService(consoleConfig))
	if svcErr != nil {
		logrus.Infof("service error: %v", svcErr)
		allErrors = append(allErrors, svcErr)
	}
	toUpdate = toUpdate || svcChanged


	// apply route
	rt, rtChanged, rtErr := routesub.ApplyRoute(co.routeClient, routesub.DefaultRoute(consoleConfig))
	if rtErr != nil {
		logrus.Infof("route error: %v", rtErr)
		allErrors = append(allErrors, rtErr)
	}
	toUpdate = toUpdate || rtChanged


	// apply configmap (needs route)
	cm, cmChanged, cmErr := resourceapply.ApplyConfigMap(co.configMapClient, configmapsub.DefaultConfigMap(consoleConfig, rt))
	if cmErr != nil {
		logrus.Infof("cm error: %v", cmErr)
		allErrors = append(allErrors, cmErr)
	}
	toUpdate = toUpdate || cmChanged


	// shared
	sharedOAuthSecretBits := crypto.Random256BitsString()


	// apply oauth (needs route)
	defaultOauthClient := oauthsub.RegisterConsoleToOAuthClient(oauthsub.DefaultOauthClient(), rt, sharedOAuthSecretBits)
	oa, oauthChanged, oauthErr := oauthsub.ApplyOAuth(co.oauthClient, defaultOauthClient)
	if oauthErr != nil {
		logrus.Infof("oauth error: %v", oauthErr)
		allErrors = append(allErrors, oauthErr)
	}
	toUpdate = toUpdate || oauthChanged


	// apply secret
	sec, secretChanged, secErr := resourceapply.ApplySecret(co.secretsClient, secret.DefaultSecret(consoleConfig,sharedOAuthSecretBits))
	if secErr != nil {
		logrus.Infof("sec error: %v", secErr)
		allErrors = append(allErrors, secErr)
	}
	toUpdate = toUpdate || secretChanged


	// apply deployment is a bit more involved as it needs information about version & if we should
	// force a rollout of the pods.  at this point, configMap updates are the bool for this
	defaultDeployment := deploymentsub.DefaultDeployment(consoleConfig)
	versionAvailability := &operatorv1alpha1.VersionAvailability{
		Version: consoleConfig.Spec.Version,
	}
	deploymentGeneration := resourcemerge.ExpectedDeploymentGeneration(defaultDeployment, versionAvailability)
	// if anything changed, we can roll out the pods again to ensure latest
	redeployPods := toUpdate
	dep, dpChanged, depErr := resourceapply.ApplyDeployment(co.deploymentClient, defaultDeployment, deploymentGeneration, redeployPods)
	if depErr != nil {
		logrus.Infof("dep error: %v", depErr)
		allErrors = append(allErrors, depErr)
	}
	toUpdate = toUpdate || dpChanged



	// if any of our resources have svcChanged, we should update the CR. otherwise, skip this step.
	if toUpdate {
		logrus.Infof("Sync_v400: To update Spec? %v", toUpdate)
		setStatus(consoleConfig.Status, svc, rt, cm, dep, oa, sec)
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

func setStatus(cs v1alpha1.ConsoleStatus, svc *corev1.Service, rt *routev1.Route, cm *corev1.ConfigMap, dep *appsv1.Deployment, oa *oauthv1.OAuthClient, sec *corev1.Secret) {
	if rt.Spec.Host != "" {
		// we have a host, yay
	} else {
		// no host, boo.
	}
	// what else.
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
