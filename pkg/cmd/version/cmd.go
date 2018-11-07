package version

import (
	"fmt"
	"strings"
	"github.com/blang/semver"
	"github.com/spf13/cobra"
)

var (
	Raw = "v0.0.1"
	Version = semver.MustParse(strings.TrimLeft(Raw, "v"))
	String = fmt.Sprintf("ConsoleOperator %s", Raw)
)

func NewVersion() *cobra.Command {
	cmd := &cobra.Command{
		Use: "version",
		Short: "Display the Operator Version",
		Run: func(command *cobra.Command, args []string) {
			fmt.Println(String)
		},
	}
	return cmd
}
