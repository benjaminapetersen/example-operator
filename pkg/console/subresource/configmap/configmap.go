package configmap

import (
	"fmt"
	"github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	"github.com/openshift/console-operator/pkg/controller"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
)

const (
	ConsoleConfigMapName    = "console-config"
	consoleConfigYamlFile   = "console-config.yaml"
	clientSecretFilePath    = "/var/oauth-config/clientSecret"
	oauthEndpointCAFilePath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	// TODO: should this be configurable?  likely so.
	documentationBaseURL = "https://docs.okd.io/4.0/"
	brandingDefault      = "okd"
	// serving info
	certFilePath = "/var/serving-cert/tls.crt"
	keyFilePath  = "/var/serving-cert/tls.key"
)

func DefaultConfigMap(cr *v1alpha1.Console, rt *v1.Route) *corev1.ConfigMap {
	if rt == nil {
		// without a route, the configmap is useless.
		// we should log, set the CR.status, and possibly create an Event
		// if we can't create this. Its a big deal.
		// (and update again once it's working)
		// also, maybe we do get a route, but if it has no host,
		// default or custom (or both or neither) then we need to
		// handle cr.Status on that as well.  That can be done
		// in context of route, not here.
		return nil
	}
	host := rt.Spec.Host
	config := NewYamlConfigString(host)
	configMap := Stub()
	configMap.Data =  map[string]string{
		consoleConfigYamlFile: config,
	}
	util.AddOwnerRef(configMap, util.OwnerRefFrom(cr))
	return configMap
}

func Stub() *corev1.ConfigMap {
	meta := util.SharedMeta()
	meta.Name = ConsoleConfigMapName
	configMap := &corev1.ConfigMap{
		ObjectMeta: meta,
	}
	return configMap
}

func NewYamlConfigString(host string) string {
	return string(NewYamlConfig(host))
}

func NewYamlConfig(host string) []byte {
	conf := yaml.MapSlice{
		{
			Key: "kind", Value: "ConsoleConfig",
		}, {
			Key: "apiVersion", Value: "console.openshift.io/v1beta1",
		}, {
			Key: "auth", Value: authServerYaml(),
		}, {
			Key: "clusterInfo", Value: clusterInfo(host),
		}, {
			Key: "customization", Value: customization(),
		}, {
			Key: "servingInfo", Value: servingInfo(),
		},
	}
	yml, err := yaml.Marshal(conf)
	if err != nil {
		fmt.Printf("Could not create config yaml %v", err)
		return nil
	}
	return yml
}

func servingInfo() yaml.MapSlice {
	return yaml.MapSlice{
		{
			Key: "bindAddress", Value: "https://0.0.0.0:8443",
		}, {
			Key: "certFile", Value: certFilePath,
		}, {
			Key: "keyFile", Value: keyFilePath,
		},
	}
}

// TODO: take args as we update branding based on cluster config?
func customization() yaml.MapSlice {
	return yaml.MapSlice{
		{
			Key: "branding", Value: brandingDefault,
		}, {
			Key: "documentationBaseURL", Value: documentationBaseURL,
		},
	}
}

//// TODO: this can take args as we update locations after we generate a router
func clusterInfo(host string) yaml.MapSlice {
	return yaml.MapSlice{
		{
			Key: "consoleBaseAddress", Value: consoleBaseAddr(host),
		}, {
			Key: "consoleBasePath", Value: "",
		},
		// {
		//   Key: "masterPublicURL", Value: nil,
		// },
	}

}

func authServerYaml() yaml.MapSlice {
	return yaml.MapSlice{
		{
			Key: "clientID", Value: controller.OpenShiftConsoleName,
			// Key: "clientID", Value: OAuthClientName,
		}, {
			Key: "clientSecretFile", Value: clientSecretFilePath,
		}, {
			Key: "logoutRedirect", Value: "",
		}, {
			Key: "oauthEndpointCAFile", Value: oauthEndpointCAFilePath,
		},
	}
}

func consoleBaseAddr(host string) string {
	if host != "" {
		str := fmt.Sprintf("https://%s", host)
		logrus.Infof("console configmap base addr set to %v", str)
		return str
	}
	return ""
}


