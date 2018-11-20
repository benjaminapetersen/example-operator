package oauthclient

import (
	"fmt"
	oauthv1 "github.com/openshift/api/oauth/v1"
	"github.com/openshift/api/route/v1"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	"github.com/openshift/console-operator/pkg/controller"
	"github.com/openshift/console-operator/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

// TODO: ApplyOauth should be a generic Apply that could be used for any oauth-client
// - should look like resourceapply.ApplyService
// - perhaps should be PR'd to client-go
func ApplyOAuth(client oauthclient.OAuthClientsGetter, required *oauthv1.OAuthClient) (*oauthv1.OAuthClient, bool, error) {
	existing, err := client.OAuthClients().Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.OAuthClients().Create(required)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}
	// Unfortunately data is all top level so its a little more
	// tedious to manually copy things over
	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	existing.Secret = required.Secret
	existing.AdditionalSecrets = required.AdditionalSecrets
	existing.RespondWithChallenges = required.RespondWithChallenges
	existing.RedirectURIs = required.RedirectURIs
	existing.GrantMethod = required.GrantMethod
	existing.ScopeRestrictions = required.ScopeRestrictions
	existing.AccessTokenMaxAgeSeconds = required.AccessTokenMaxAgeSeconds
	existing.AccessTokenInactivityTimeoutSeconds = required.AccessTokenInactivityTimeoutSeconds

	actual, err := client.OAuthClients().Update(existing)
	return actual, true, err
}

// registers the console on the oauth client as a valid application
func RegisterConsoleToOAuthClient(client *oauthv1.OAuthClient, route *v1.Route, randomBits string) *oauthv1.OAuthClient {
	// without a route, we cannot create a usable oauth client
	if route == nil {
		return nil
	}
	// we should be the only ones using this client, so we can
	// stomp all over existing RedirectURIs.
	// TODO: potentially support multiple if multiple routes service
	// the console
	client.RedirectURIs = []string{}
	client.RedirectURIs = append(client.RedirectURIs, https(route.Spec.Host))
	// client.Secret = randomBits
	client.Secret = string(randomBits)
	return client
}

// for ManagementState.Removed
func DeRegisterConsoleFromOAuthClient(client *oauthv1.OAuthClient) *oauthv1.OAuthClient {
	client.RedirectURIs = []string{}
	// changing the string to anything else will invalidate the client
	client.Secret = crypto.Random256BitsString()
	return client
}

// cr *v1alpha1.Console, rt *v1.Route
// the OAuthClient is a cluster scoped resource that will be stamped
// out on install by the CVO.  We know for certain we will not create
// this, so there is no point in fleshing out its values here, unlike
// the other resources we are responsible for.
func DefaultOauthClient() *oauthv1.OAuthClient{
	// we cannot set an ownerRef on the OAuthClient as it is
	// a cluster scoped resource.
	return &oauthv1.OAuthClient{
		ObjectMeta: metav1.ObjectMeta{
			Name: controller.OpenShiftConsoleName,
		},
		// we can't really set these here yet but need them
		// RedirectURIs: []string{},
		// Secret: crypto.Random256BitsString(),
	}
}

// TODO: technically, this should take targetPort from route.spec.port.targetPort
func https(host string) string {
	protocol := "https://"
	if host == "" {
		logrus.Infof("host is invalid empty string")
		return ""
	}
	if strings.HasPrefix(host, protocol) {
		return host
	}
	secured := fmt.Sprintf("%s%s", protocol, host)
	logrus.Infof("host updated from %s to %s", host, secured)
	return secured
}

func GetSecretString(client *oauthv1.OAuthClient) string {
	return client.Secret
}
