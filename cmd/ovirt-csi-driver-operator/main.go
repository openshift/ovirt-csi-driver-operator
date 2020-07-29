package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	k8sflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	"github.com/ovirt/csi-driver-operator/pkg/operator"
	"github.com/ovirt/csi-driver-operator/pkg/version"
)

var nodeName string

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	pflag.CommandLine.SetNormalizeFunc(k8sflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	logs.InitLogs()
	defer logs.FlushLogs()

	command := NewOperatorCommand()
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func NewOperatorCommand() *cobra.Command {
	op, _ := operator.NewCSIOperator(&nodeName)

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
	).NewCommand()
	ctrlCmd.Use = "start"
	ctrlCmd.Short = "Start the oVirt CSI Driver Operator"
	ctrlCmd.Flags().StringVar(&nodeName, "node", "", "kubernetes node name")
	cmd.AddCommand(ctrlCmd)

	return cmd
}
