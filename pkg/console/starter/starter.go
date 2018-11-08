package starter

import (
	"fmt"
	"time"

	"k8s.io/client-go/tools/cache"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/library-go/pkg/operator/status"
	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	"github.com/openshift/console-operator/pkg/console/operator"
	"github.com/openshift/console-operator/pkg/generated/clientset/versioned"
	"github.com/openshift/console-operator/pkg/generated/informers/externalversions"
)


// Time to wire up our informers/clients/etc
//
// informers
// - listen for changes
//
// clients
// - used by informers to get,list,put resources
//
// informers
// - instantiated in starter.go (here)
// - consumed in... places.
func RunOperator(clientConfig *rest.Config, stopCh <-chan struct{}) error {

	fmt.Printf("starter.go -> RunOperator() func........\n")

	// creates a new kube clientset
	// clientConfig is a REST config
	// a clientSet contains clients for groups.
	// each group has one version included in the set.
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	// pkg/apis/console/v1alpha1/types.go has a `genclient` annotation,
	// that creates the expected functions for the type.
	consoleOperatorClient, err := versioned.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	const resync = 10 * time.Minute

	// only watch a specific resource name
	// this is an optimization step that is not initially needed
	// TODO: eliminate for now?
	tweakListOptions := func(options *v1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector("metadata.name", operator.ResourceName).String()
	}

	kubeInformersNamespaced := informers.NewSharedInformerFactoryWithOptions(
		// takes a client
		kubeClient,
		resync,
		// takes an unlimited number of additional "options" arguments, which are functions,
		// that take a sharedInformerFactory and return a sharedInformerFactory
		informers.WithNamespace(operator.TargetNamespace),
		informers.WithTweakListOptions(tweakListOptions),
	)

	consoleOperatorInformers := externalversions.NewSharedInformerFactoryWithOptions(
		// this is our generated client
		consoleOperatorClient,
		resync,
		// and the same set of optional transform functions
		externalversions.WithNamespace(operator.TargetNamespace),
		externalversions.WithTweakListOptions(tweakListOptions),
	)

	consoleOperator := operator.NewConsoleOperator(
		// informer factory, drilling down to the v1alpha1 console informer
		consoleOperatorInformers.Console().V1alpha1().Consoles(),
		// specifically secrets, we can add more here.
		kubeInformersNamespaced.Core().V1().Secrets(),
		// client and informer are NOT the same
		consoleOperatorClient.ConsoleV1alpha1().Consoles(operator.TargetNamespace),
		// client and informer are NOT the same
		kubeClient.CoreV1(),
	)

	kubeInformersNamespaced.Start(stopCh)
	consoleOperatorInformers.Start(stopCh)

	go consoleOperator.Run(stopCh)

	// prob move this up...
	dynamicClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	// TODO: create the status  here....
	// to test this, will need to update STATUS on my CR
	// and then check and see if it is updated on the other deal.
	// TODO: install this, from the old console-operator install  operatorstatus.openshift.io
	clusterOperatorStatus := status.NewClusterOperatorStatusController(
		// TODO: these may not be exactly right...
		// make sure they are what we actually need.
		operator.TargetNamespace,
		operator.ResourceName,
		// for some reason we need a dynamic client.  this is weird, and i dont know why
		// given that we rant about "real" strongly typed clients constantly.
		dynamicClient,
		&operatorStatusProvider{informers: consoleOperatorInformers},
	)
	// TODO: will have a series of Run() funcs here
	go clusterOperatorStatus.Run(1, stopCh)

	<-stopCh

	return fmt.Errorf("stopped")
}


// NOTE: i want this in a separate package,
// but most other operators seem to keep it here :/
type operatorStatusProvider struct {
	informers externalversions.SharedInformerFactory
}

func (p *operatorStatusProvider) Informer() cache.SharedIndexInformer {
	return p.informers.Console().V1alpha1().Consoles().Informer()
}

func (p *operatorStatusProvider) CurrentStatus() (operatorv1alpha1.OperatorStatus, error) {
	instance, err := p.informers.Console().V1alpha1().Consoles().Lister().Consoles(operator.TargetNamespace).Get(operator.ResourceName)
	if err != nil {
		return operatorv1alpha1.OperatorStatus{}, err
	}

	return instance.Status.OperatorStatus, nil
}

