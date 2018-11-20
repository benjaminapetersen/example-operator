package operator

import (
	// 3rd party
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	// kube
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	errutil "k8s.io/apimachinery/pkg/util/errors"
	oauthv1 "github.com/openshift/api/oauth/v1"
	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	oauthclientv1 "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	// openshift
	"github.com/openshift/console-operator/pkg/controller"
	"github.com/openshift/console-operator/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	// operator
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	configmapsub "github.com/openshift/console-operator/pkg/console/subresource/configmap"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
	oauthsub "github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
	secretsub "github.com/openshift/console-operator/pkg/console/subresource/secret"
	servicesub "github.com/openshift/console-operator/pkg/console/subresource/service"
)

// runs the standard v4.0.0 reconcile loop
// the Apply logic is a bit tricky.
// - Default Route is incomplete, we expect the server to fill out host,
//   so should not stomp on it after the initial run
// - other resources can be a tad tricky as well.
func sync_v400(co *ConsoleOperator, consoleConfig *v1alpha1.Console) (*v1alpha1.Console, error) {
	// aggregate the errors from this:
	allErrors := []error{}
	toUpdate := false



	// apply service
	_, svcChanged, svcErr := resourceapply.ApplyService(co.serviceClient, servicesub.DefaultService(consoleConfig))
	if svcErr != nil {
		logrus.Errorf("service error: %v", svcErr)
		allErrors = append(allErrors, svcErr)
	}
	toUpdate = toUpdate || svcChanged



	// apply route
	// TODO:
	// - DefaultRoute() should literally just be *Default*
	// - get the Default().
	//   - if exists, great. if not, Create()
	//   - this would avoid the stomping if we use Apply() as it won't merge back...
	// - EnsureRouteSpec()
	//   - check that everything we need is there
	// - ApplyRoute()
	//   - the job of ApplyRoute() is to ensure exists, then update correctly via
	//     merge.  This *correctly* is generic, not specific to what OUR route needs.
	//   - therefore, it is appropriate to split apart EnsureRouteSpec() from ApplyRoute()
	//     - note that EnsureRouteSpec() may not be great long term, depends...
	rt, rtIsNew, rtErr := routesub.GetOrCreate(co.routeClient, routesub.DefaultRoute(consoleConfig))
	// rt, rtChanged, rtErr := routesub.ApplyRoute(co.routeClient, routesub.DefaultRoute(consoleConfig))
	if rtErr != nil {
		logrus.Errorf("route error: %v", rtErr)
		allErrors = append(allErrors, rtErr)
	}
	toUpdate = toUpdate || rtIsNew



	// apply configmap (needs route)
	_, cmChanged, cmErr := resourceapply.ApplyConfigMap(co.configMapClient, configmapsub.DefaultConfigMap(consoleConfig, rt))
	if cmErr != nil {
		logrus.Errorf("cm error: %v", cmErr)
		allErrors = append(allErrors, cmErr)
	}
	toUpdate = toUpdate || cmChanged

	// the deployment will need to know if the secret changed so this must be func scoped
	secretChanged := false
	oauthChanged := false
	if !secretsMatch(co.secretsClient, co.oauthClient) {
		// shared secret bits
		// sharedOAuthSecretBits := crypto.RandomBits(256)
		sharedOAuthSecretBits := crypto.Random256BitsString()

		// apply oauth (needs route)
		defaultOauthClient := oauthsub.RegisterConsoleToOAuthClient(oauthsub.DefaultOauthClient(), rt, sharedOAuthSecretBits)
		_, oauthChanged, oauthErr := oauthsub.ApplyOAuth(co.oauthClient, defaultOauthClient)
		if oauthErr != nil {
			logrus.Errorf("oauth error: %v", oauthErr)
			allErrors = append(allErrors, oauthErr)
		}
		toUpdate = toUpdate || oauthChanged

		// apply secret
		_, secretChanged, secErr := resourceapply.ApplySecret(co.secretsClient, secretsub.DefaultSecret(consoleConfig, sharedOAuthSecretBits))
		if secErr != nil {
			logrus.Errorf("sec error: %v", secErr)
			allErrors = append(allErrors, secErr)
		}
		toUpdate = toUpdate || secretChanged

	}

	// TODO: deployment changes too much, dont trigger loop.
	// the Apply() again is prob incorrect in that our DefaultDeploymnet() is
	// too much... and there is prob something that gets stomped.
	// apply deployment is a bit more involved as it needs information about version & if we should
	// force a rollout of the pods.  at this point, configMap updates are the bool for this
	defaultDeployment := deploymentsub.DefaultDeployment(consoleConfig)
	versionAvailability := &operatorv1alpha1.VersionAvailability{
		Version: consoleConfig.Spec.Version,
	}
	deploymentGeneration := resourcemerge.ExpectedDeploymentGeneration(defaultDeployment, versionAvailability)
	// if configMap or secrets change, we need to deploy a new pod
	redeployPods := cmChanged || secretChanged
	_, depChanged, depErr := resourceapply.ApplyDeployment(co.deploymentClient, defaultDeployment, deploymentGeneration, redeployPods)
	if depErr != nil {
		logrus.Infof("dep error: %v", depErr)
		allErrors = append(allErrors, depErr)
	}
	toUpdate = toUpdate || depChanged



	logrus.Printf("service changed: %v \n", svcChanged)
	logrus.Printf("route is new: %v \n", rtIsNew)
	logrus.Printf("configMap changed: %v \n", cmChanged)
	logrus.Printf("secret changed: %v \n", secretChanged)
	logrus.Printf("oauth changed: %v \n", oauthChanged)
	logrus.Printf("deployment changed: %v \n", depChanged)
	logrus.Println("------------")

	// if any of our resources have svcChanged, we should update the CR. otherwise, skip this step.
	if toUpdate {
		logrus.Infof("Sync_v400: To update Spec? %v", toUpdate)
		// TODO: set the status.
		// setStatus(consoleConfig.Status, svc, rt, cm, dep, oa, sec)
	}

	return consoleConfig, errutil.FilterOut(errutil.NewAggregate(allErrors), apierrors.IsNotFound)
}


func secretsMatch(secretGetter coreclientv1.SecretsGetter, clientsGetter oauthclientv1.OAuthClientsGetter) bool {
	secret, err := secretGetter.Secrets(controller.TargetNamespace).Get(deploymentsub.ConsoleOauthConfigName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false
	}
	oauthClient, err := clientsGetter.OAuthClients().Get(controller.OpenShiftConsoleName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false
	}

	return secretAndOauthMatch(secret, oauthClient)
}

func secretAndOauthMatch(secret *corev1.Secret, client *oauthv1.OAuthClient) bool {
	secretString := secretsub.GetSecretString(secret)
	clientSecretString := oauthsub.GetSecretString(client)
	return secretString == clientSecretString
}


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
func setStatus(cs v1alpha1.ConsoleStatus, svc *corev1.Service, rt *routev1.Route, cm *corev1.ConfigMap, dep *appsv1.Deployment, oa *oauthv1.OAuthClient, sec *corev1.Secret) {
	logrus.Println("setStatus()")
	logrus.Println("-----------")

	// TODO: handle custom hosts as well
	if rt.Spec.Host != "" {
		cs.DefaultHostName = rt.Spec.Host
		logrus.Printf("stats.DefaultHostName set to %v", rt.Spec.Host)
	} else {
		cs.DefaultHostName = ""
		logrus.Printf("stats.DefaultHostName set to %v", "")
	}

	if secretAndOauthMatch(sec, oa) {
		cs.OAuthSecret = "valid"
		logrus.Printf("status.OAuthSecret is valid")
	} else {
		cs.OAuthSecret = "mismatch"
		logrus.Printf("status.OAuthSecret is mismatch")
	}

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
