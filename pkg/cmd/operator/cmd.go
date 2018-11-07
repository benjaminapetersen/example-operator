package operator

import (
	// 3rd party
	"github.com/spf13/cobra"
	// kube / openshift
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	// us
	"github.com/openshift/console-operator/pkg/console/starter"
	"github.com/openshift/console-operator/pkg/console/version"
)

func NewOperator() *cobra.Command {
	cmd := controllercmd.
		NewControllerCommandConfig("console-operator", version.Get(), starter.RunOperator).
		NewCommand()
	cmd.Use = "operator"
	cmd.Short = "Start the Console Operator"

	return cmd
}
