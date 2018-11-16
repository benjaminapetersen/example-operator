package secret

import (
	"k8s.io/api/core/v1"
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/openshift/console-operator/pkg/console/subresource/deployment"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
)

const dataKey = "clientsecret"

func DefaultSecret(cr *v1alpha1.Console, randomBits string) *v1.Secret {
	meta := util.SharedMeta()
	meta.Name = deployment.ConsoleOauthConfigName
	secret := &v1.Secret{
		ObjectMeta: meta,
	}
	secret.StringData = map[string]string{
		dataKey: randomBits,
	}
	util.AddOwnerRef(secret, util.OwnerRefFrom(cr))
	return secret
}
