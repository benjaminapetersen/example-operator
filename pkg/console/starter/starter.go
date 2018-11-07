package starter

import (
	"fmt"
	"github.com/openshift/library-go/pkg/operator/status"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/console-operator/pkg/console/operator"
	"github.com/openshift/console-operator/pkg/generated/clientset/versioned"
	"github.com/openshift/console-operator/pkg/generated/informers/externalversions"
)

func RunOperator(clientConfig *rest.Config, stopCh <-chan struct{}) error {
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	consoleOperatorClient, err := versioned.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	const resync = 10 * time.Minute

	// only watch a specific resource name
	tweakListOptions := func(options *v1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector("metadata.name", operator.ResourceName).String()
	}

	kubeInformersNamespaced := informers.NewSharedInformerFactoryWithOptions(kubeClient, resync,
		informers.WithNamespace(operator.TargetNamespace),
		informers.WithTweakListOptions(tweakListOptions),
	)

	consoleOperatorInformers := externalversions.NewSharedInformerFactoryWithOptions(consoleOperatorClient, resync,
		externalversions.WithNamespace(operator.TargetNamespace),
		externalversions.WithTweakListOptions(tweakListOptions),
	)

	consoleOperator := operator.NewConsoleOperator(
		consoleOperatorInformers.Console().V1alpha1().Consoles(),
		kubeInformersNamespaced.Core().V1().Secrets(),
		consoleOperatorClient.ConsoleV1alpha1().Consoles(operator.TargetNamespace),
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
	//clusterOperatorStatus := status.NewClusterOperatorStatusController(
	//	// what are these? name of controller(operator)? wire this up.
	//	"openshift-apiserver",
	//	"openshift-apiserver",
	//	// what is this
	//	dynamicClient,
	//	&operatorStatusProvider{informers: operatorConfigInformers},
	//)
	//
	//// TODO: will have a series of runs here
	//go clusterOperatorStatus.Run(1, stopCh)

	<-stopCh

	return fmt.Errorf("stopped")
}



// Interface
//type operatorStatusProvider struct {
//	informers operatorclientinformers.SharedInformerFactory
//}
//
//func (p *operatorStatusProvider) Informer() cache.SharedIndexInformer {
//	// TODO: return my informaer....
//	// return p.informers.Openshiftapiserver().V1alpha1().OpenShiftAPIServerOperatorConfigs().Informer()
//}
//
//func (p *operatorStatusProvider) CurrentStatus() (operatorv1alpha1.OperatorStatus, error) {
//	// TODO: use my informer to get my status
//	instance, err := p.informers.Openshiftapiserver().V1alpha1().OpenShiftAPIServerOperatorConfigs().Lister().Get("instance")
//	if err != nil {
//		return operatorv1alpha1.OperatorStatus{}, err
//	}
//
//	return instance.Status.OperatorStatus, nil
//}
