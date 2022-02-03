package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"k8s.io/component-base/cli"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	"github.com/ovirt/csi-driver-operator/pkg/operator"
	"github.com/ovirt/csi-driver-operator/pkg/version"
)

var nodeName string

func main() {

	command := NewOperatorCommand()
	code := cli.Run(command)
	os.Exit(code)
}

func NewOperatorCommand() *cobra.Command {
	op, err := operator.NewCSIOperator(&nodeName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	cmd := &cobra.Command{
		Use:   "ovirt-csi-driver-operator",
		Short: "OpenShift oVirt CSI Driver Operator",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			os.Exit(1)
		},
	}
	ctrlCmd := controllercmd.NewControllerCommandConfig(
		"ovirt-csi-driver-operator",
		version.Get(),
		op.RunOperator,
	).NewCommandWithContext(context.Background())
	ctrlCmd.Use = "start"
	ctrlCmd.Short = "Start the oVirt CSI Driver Operator"
	ctrlCmd.Flags().StringVar(&nodeName, "node", "", "kubernetes node name")
	cmd.AddCommand(ctrlCmd)

	return cmd
}
