package status

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type DiscoverOptions struct {
	// ShowOtherConditions is a list of comma separated kind or kind/name for which we should add   the ShowObjectConditionsAnnotation
	// to signal to the presentation layer to show all the conditions for the objects.
	ShowOtherConditions string

	// DisableNoEcho disable hiding MachineInfrastructure or BootstrapConfig objects if the object's ready condition is true
	// or it has the same Status, Severity and Reason of the parent's object ready condition (it is an echo)
	DisableNoEcho bool

	// DisableGroupObjects disable grouping machines objects in case the ready condition
	// has the same Status, Severity and Reason
	DisableGroupObjects bool
}

func (d DiscoverOptions) toObjectTreeOptions() objectTreeOptions {
	return objectTreeOptions{
		ShowOtherConditions: d.ShowOtherConditions,
		DisableNoEcho:       d.DisableNoEcho,
		DisableGroupObjects: d.DisableGroupObjects,
	}
}

func Discovery(ctx context.Context, c client.Client, cluster *clusterv1.Cluster, options DiscoverOptions) (*ObjectTree, error) {
	objs := newObjectTree(options.toObjectTreeOptions())

	clusterInfra, err := external.Get(ctx, c, cluster.Spec.InfrastructureRef, cluster.Namespace)
	// TODO: consider for a proper error management for reading ref; however, during the delete workflow it might be correct a ref does not exist...
	if err == nil {
		objs.add(cluster, clusterInfra, ObjectMetaName("ClusterInfrastructure"))
	}

	controlPLane, err := external.Get(ctx, c, cluster.Spec.ControlPlaneRef, cluster.Namespace)
	if err == nil {
		objs.add(cluster, controlPLane, ObjectMetaName("ControlPlane"), GroupingObject(true))
	}

	machinesList, err := getMachinesInCluster(ctx, c, cluster.Namespace, cluster.Name)
	if err != nil {
		return nil, err
	}
	machineMap := map[string]bool{}
	addMachineFunc := func(parent controllerutil.Object, m *clusterv1.Machine) {
		objs.add(parent, m)
		machineMap[m.Name] = true

		machineInfra, err := external.Get(ctx, c, &m.Spec.InfrastructureRef, cluster.Namespace)
		// TODO:error management. In some case it is ok the object is missing
		if err == nil {
			objs.add(m, machineInfra, ObjectMetaName("MachineInfrastructure"), NoEcho(true))
		}

		machineBootstrap, err := external.Get(ctx, c, m.Spec.Bootstrap.ConfigRef, cluster.Namespace)
		// TODO:error management. In some case it is ok the object is missing
		if err == nil {
			objs.add(m, machineBootstrap, ObjectMetaName("BootstrapConfig"), NoEcho(true))
		}
	}

	controlPlaneMachines := selectControlPlaneMachines(machinesList)
	for i := range controlPlaneMachines {
		cp := controlPlaneMachines[i]
		addMachineFunc(controlPLane, cp)
	}

	if len(machinesList.Items) == len(controlPlaneMachines) {
		return objs, nil
	}

	workers := virtualObject(cluster.Namespace, "Workers")
	objs.add(cluster, workers)

	machinesDeploymentList, err := getMachineDeploymentsInCluster(ctx, c, cluster.Namespace, cluster.Name)
	if err != nil {
		return nil, err
	}

	machineSetList, err := getMachineSetsInCluster(ctx, c, cluster.Namespace, cluster.Name)
	if err != nil {
		return nil, err
	}

	for i := range machinesDeploymentList.Items {
		md := &machinesDeploymentList.Items[i]
		objs.add(workers, md, GroupingObject(true))

		machineSets := selectMachinesSetsControlledBy(machineSetList, md)
		for i := range machineSets {
			ms := machineSets[i]

			machines := selectMachinesControlledBy(machinesList, ms)
			for _, w := range machines {
				addMachineFunc(md, w)
			}
		}
	}

	if len(machineMap) < len(machinesList.Items) {
		other := virtualObject(cluster.Namespace, "Other")
		objs.add(workers, other)

		for i := range machinesList.Items {
			m := &machinesList.Items[i]
			if _, ok := machineMap[m.Name]; ok {
				continue
			}
			addMachineFunc(other, m)
		}
	}

	return objs, nil
}

func getMachinesInCluster(ctx context.Context, c client.Client, namespace, name string) (*clusterv1.MachineList, error) {
	if name == "" {
		return nil, nil
	}

	machineList := &clusterv1.MachineList{}
	labels := map[string]string{clusterv1.ClusterLabelName: name}

	if err := c.List(ctx, machineList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	return machineList, nil
}

func getMachineDeploymentsInCluster(ctx context.Context, c client.Client, namespace, name string) (*clusterv1.MachineDeploymentList, error) {
	if name == "" {
		return nil, nil
	}

	machineDeploymentList := &clusterv1.MachineDeploymentList{}
	labels := map[string]string{clusterv1.ClusterLabelName: name}

	if err := c.List(ctx, machineDeploymentList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	return machineDeploymentList, nil
}

func getMachineSetsInCluster(ctx context.Context, c client.Client, namespace, name string) (*clusterv1.MachineSetList, error) {
	if name == "" {
		return nil, nil
	}

	machineSetList := &clusterv1.MachineSetList{}
	labels := map[string]string{clusterv1.ClusterLabelName: name}

	if err := c.List(ctx, machineSetList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	return machineSetList, nil
}

func selectControlPlaneMachines(machineList *clusterv1.MachineList) []*clusterv1.Machine {
	machines := []*clusterv1.Machine{}
	for i := range machineList.Items {
		m := &machineList.Items[i]
		if util.IsControlPlaneMachine(m) {
			machines = append(machines, m)
		}
	}
	return machines
}

func selectMachinesSetsControlledBy(machineSetList *clusterv1.MachineSetList, controller controllerutil.Object) []*clusterv1.MachineSet {
	machineSets := []*clusterv1.MachineSet{}
	for i := range machineSetList.Items {
		m := &machineSetList.Items[i]
		if util.IsControlledBy(m, controller) {
			machineSets = append(machineSets, m)
		}
	}
	return machineSets
}

func selectMachinesControlledBy(machineList *clusterv1.MachineList, controller controllerutil.Object) []*clusterv1.Machine {
	machines := []*clusterv1.Machine{}
	for i := range machineList.Items {
		m := &machineList.Items[i]
		if util.IsControlledBy(m, controller) {
			machines = append(machines, m)
		}
	}
	return machines
}
