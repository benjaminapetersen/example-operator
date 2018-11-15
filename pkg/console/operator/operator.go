package operator

import (
	"fmt"

	"github.com/blang/semver"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers/core/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	operatorsv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/versioning"
	// informers
	routesinformersv1 "github.com/openshift/client-go/route/informers/externalversions/route/v1"
	appsinformersv1 "k8s.io/client-go/informers/apps/v1"
	oauthinformersv1 "github.com/openshift/client-go/oauth/informers/externalversions/oauth/v1"
	consoleinformers "github.com/openshift/console-operator/pkg/generated/informers/externalversions/console/v1alpha1"
	// clients
	"github.com/openshift/console-operator/pkg/generated/clientset/versioned/typed/console/v1alpha1"
	routeclientv1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	oauthclientv1 "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	// operator
	"github.com/openshift/console-operator/pkg/controller"
	consolev1alpha1 "github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
)

const (
	// TargetNamespace could be made configurable if desired
	TargetNamespace = "openshift-console"

	// ResourceName could be made configurable if desired
	// all resources share the same name to make it easier to reason about and to configure single item watches
	// NOTE: this must match metadata.name in the CR.yaml else the CR will be ignored
	ResourceName = "console-operator-resource"

	// workQueueKey is the singleton key shared by all events
	// the value is irrelevant
	workQueueKey = "console-operator-queue-key"
)

// consts to maintain existing names of various sub-resources
const (
	OpenShiftConsoleName      = "openshift-console"
	OpenShiftConsoleShortName = "console"
	OpenShiftConsoleNamespace = "openshift-console"
)

// func NewConsoleOperator(
// 	coi consoleinformers.ConsoleOperatorInformer,
// 	si v1.SecretInformer,
// 	operatorClient v1alpha1.ConsoleOperatorInterface,
// 	secretsClient coreclientv1.SecretsGetter) *ConsoleOperator {
func NewConsoleOperator(
	// informers
	coi consoleinformers.ConsoleInformer,
	coreV1 v1.Interface,
	appsV1 appsinformersv1.Interface,
	routesV1 routesinformersv1.Interface,
	oauthV1 oauthinformersv1.Interface,
	// clients
	operatorClient v1alpha1.ConsoleInterface,
	corev1Client coreclientv1.CoreV1Interface,
	appsv1Client appsv1.AppsV1Interface,
	routev1Client routeclientv1.RouteV1Interface,
	oauthv1Client oauthclientv1.OauthV1Interface) *ConsoleOperator {


	c := &ConsoleOperator{
		// operator
		operatorClient: operatorClient,
		// core kube
		secretsClient:  corev1Client,
		configMapClient: corev1Client,
		serviceClient: corev1Client,
		deploymentClient: appsv1Client,
		// openshift
		routeClient: routev1Client,
		oauthClient: oauthv1Client,
	}

	operatorInformer := coi.Informer()
	secretsInformer := coreV1.Secrets().Informer()
	deploymentInformer := appsV1.Deployments().Informer()
	configMapInformer := coreV1.ConfigMaps().Informer()
	serviceInformer := coreV1.Services().Informer()
	routeInformer := routesV1.Routes().Informer()
	oauthInformer := oauthV1.OAuthClientAuthorizations().Informer()

	// we do not really need to wait for our informers to sync since we only watch a single resource
	// and make live reads but it does not hurt anything and guarantees we have the correct behavior
	internalController, queue := controller.New(
		"Console",
		c.sync,
		operatorInformer.HasSynced,
		secretsInformer.HasSynced,
		deploymentInformer.HasSynced,
		configMapInformer.HasSynced,
		serviceInformer.HasSynced,
		routeInformer.HasSynced,
		oauthInformer.HasSynced)

	c.controller = internalController

	operatorInformer.AddEventHandler(eventHandler(queue))
	secretsInformer.AddEventHandler(eventHandler(queue))
	deploymentInformer.AddEventHandler(eventHandler(queue))
	configMapInformer.AddEventHandler(eventHandler(queue))
	serviceInformer.AddEventHandler(eventHandler(queue))
	routeInformer.AddEventHandler(eventHandler(queue))
	oauthInformer.AddEventHandler(eventHandler(queue))

	return c
}

// eventHandler queues the operator to check spec and status
// TODO add filtering and more nuanced logic
// each informer's event handler could have specific logic based on the resource
// for now just rekicking the sync loop is enough since we only watch a single resource by name
func eventHandler(queue workqueue.RateLimitingInterface) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { queue.Add(workQueueKey) },
		UpdateFunc: func(old, new interface{}) { queue.Add(workQueueKey) },
		DeleteFunc: func(obj interface{}) { queue.Add(workQueueKey) },
	}
}

type ConsoleOperator struct {
	// for a performance sensitive operator, it would make sense to use informers
	// to handle reads and clients to handle writes.  since this operator works
	// on a singleton resource, it has no performance requirements.
	operatorClient v1alpha1.ConsoleInterface
	// core kube
	secretsClient  coreclientv1.SecretsGetter
	configMapClient coreclientv1.ConfigMapsGetter
	serviceClient coreclientv1.ServicesGetter
	deploymentClient appsv1.DeploymentsGetter
	// openshift
	routeClient routeclientv1.RoutesGetter
	oauthClient oauthclientv1.OAuthClientsGetter
	// controller
	controller *controller.Controller
}

func (c *ConsoleOperator) Run(stopCh <-chan struct{}) {
	// only start one worker because we only have one key name in our queue
	// since this operator works on a singleton, it does not make sense to ever run more than one worker
	c.controller.Run(1, stopCh)
}

// sync() is the handler() function equivalent from the sdk
// this is the big switch statement.
func (c *ConsoleOperator) sync(_ interface{}) error {
	// we ignore the passed in key because it will always be workQueueKey
	// it does not matter how the sync loop was triggered
	// all we need to worry about is reconciling the state back to what we expect
	operatorConfig, err := c.operatorClient.Get(ResourceName, metav1.GetOptions{})

	if errors.IsNotFound(err) {
		_, err := c.operatorClient.Create(c.defaultConsole())
		return err
	}
	if err != nil {
		return err
	}

	switch operatorConfig.Spec.ManagementState {
	case operatorsv1alpha1.Managed:
		// handled below
	case operatorsv1alpha1.Unmanaged:
		return nil
	case operatorsv1alpha1.Removed:
		return utilerrors.FilterOut(c.secretsClient.Secrets(TargetNamespace).Delete(ResourceName, nil), errors.IsNotFound)
	default:
		// TODO should update status
		return fmt.Errorf("unknown state: %v", operatorConfig.Spec.ManagementState)
	}

	var currentActualVerion *semver.Version

	// TODO: ca.yaml needs a version, update the v1alpha1.Console to include version field
	if ca := operatorConfig.Status.CurrentAvailability; ca != nil {
		ver, err := semver.Parse(ca.Version)
		if err != nil {
			utilruntime.HandleError(err)
		} else {
			currentActualVerion = &ver
		}
	}
	desiredVersion, err := semver.Parse(operatorConfig.Spec.Version)
	if err != nil {
		// TODO report failing status, we may actually attempt to do this in the "normal" error handling
		return err
	}
	v310_00_to_unknown := versioning.NewRangeOrDie("3.10.0", "3.10.1")

	outConfig := operatorConfig.DeepCopy()
	var errs []error


	fmt.Println("[Version]")
	fmt.Printf("Current version: %v, Desired version: %v \n", currentActualVerion, desiredVersion)
	fmt.Printf("Between or empty? %v, between? %v", v310_00_to_unknown.BetweenOrEmpty(currentActualVerion), v310_00_to_unknown.Between(&desiredVersion))
	fmt.Println("-------------")

	switch {
	// TODO: 
	// Essentially currentActualVersion & desiredVersion matches
	// our current version is <nil> so it triggers this block.
	// once we put a version (4.0.0) it will no longer trigger this block.
	// we need an actual v4
	// so
	// case is_v400
	//   reconcile v400
	// case is_v311_but_upgrading
	//   upgrade v400
	// etc.
	case v310_00_to_unknown.BetweenOrEmpty(currentActualVerion) && v310_00_to_unknown.Between(&desiredVersion):
		_, _, err := resourceapply.ApplySecret(c.secretsClient, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ResourceName,
				Namespace: TargetNamespace,
			},
			Data: map[string][]byte{
				operatorConfig.Spec.Value: []byte("007"),
			},
		})
		errs = append(errs, err)

		if err == nil { // this needs work, but good enough for now
			outConfig.Status.TaskSummary = "sync-[3.10.0,3.10.1)"
			outConfig.Status.CurrentAvailability = &operatorsv1alpha1.VersionAvailability{
				Version: desiredVersion.String(),
			}
		}

	default:
		outConfig.Status.TaskSummary = "unrecognized"
	}

	// TODO: this should do better apply logic or similar, maybe use SetStatusFromAvailability
	_, err = c.operatorClient.Update(outConfig)
	errs = append(errs, err)

	return utilerrors.NewAggregate(errs)
}


func (c *ConsoleOperator) defaultConsole() *consolev1alpha1.Console {
	return &consolev1alpha1.Console{
		ObjectMeta: metav1.ObjectMeta{
			// Name: OpenShiftConsoleName,
			Name: ResourceName,
			Namespace: OpenShiftConsoleNamespace,
		},
		Spec: consolev1alpha1.ConsoleSpec{
			OperatorSpec: operatorsv1alpha1.OperatorSpec{
				ManagementState: "Managed",
			},
		},
	}
}




