package main

import (
	"context"
	"os"

	"github.com/fabriziopandini/capi-conditions/cmd/kubectl-capi-tree/status"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var cf *genericclioptions.ConfigFlags

// This variable is populated by goreleaser
var version string

var Scheme = runtime.NewScheme()

var (
	showOtherConditions string
	disableNoEcho       bool
	disableGroupObjects bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:          "kubectl capi cluster status CLUSTER",
	SilenceUsage: true, // for when RunE returns an error
	Args:         cobra.MinimumNArgs(1),
	RunE:         run,
	Version:      versionString(),
}

func run(command *cobra.Command, args []string) error {
	ctx := context.Background()

	name := args[0]
	namespace := getNamespace()

	restConfig, err := cf.ToRESTConfig()
	if err != nil {
		return err
	}
	restConfig.QPS = 1000
	restConfig.Burst = 1000

	c, err := client.New(restConfig, client.Options{Scheme: Scheme})
	if err != nil {
		return err
	}

	// Fetch the Cluster instance.
	cluster := &clusterv1.Cluster{}
	clusterKey := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
	if err := c.Get(ctx, clusterKey, cluster); err != nil {
		return err
	}
	cluster.Kind = "Cluster" // TODO: investigate why this is empty

	// Discovery the cluster status
	objs, err := status.Discovery(ctx, c, cluster, status.DiscoverOptions{
		ShowOtherConditions: showOtherConditions,
		DisableNoEcho:       disableNoEcho,
		DisableGroupObjects: disableGroupObjects,
	})
	if err != nil {
		return err
	}

	// Output the status on the CLI
	treeView(os.Stderr, objs, cluster)

	return nil
}

// versionString returns the version prefixed by 'v'
// or an empty string if no version has been populated by goreleaser.
// In this case, the --version flag will not be added by cobra.
func versionString() string {
	if len(version) == 0 {
		return ""
	}
	return "v" + version
}

func getNamespace() string {
	if v := *cf.Namespace; v != "" {
		return v
	}
	clientConfig := cf.ToRawKubeConfigLoader()
	defaultNamespace, _, err := clientConfig.Namespace()
	if err != nil {
		defaultNamespace = "default"
	}
	return defaultNamespace
}

func init() {
	_ = clusterv1.AddToScheme(Scheme)

	cf = genericclioptions.NewConfigFlags(true)
	cf.AddFlags(rootCmd.Flags())

	rootCmd.Flags().StringVar(&showOtherConditions, "show-all-conditions", "", " list of comma separated kind or kind/name for which we should show all the object's conditions (all to show conditions for all the objects)")
	rootCmd.Flags().BoolVar(&disableNoEcho, "disable-no-echo", false, "Disable hiding of a MachineInfrastructure and BootstrapConfig when ready condition is true or it has the Status, Severity and Reason of the machine's object")
	rootCmd.Flags().BoolVar(&disableGroupObjects, "disable-grouping", false, "Disable grouping machines when ready condition has the same Status, Severity and Reason")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
