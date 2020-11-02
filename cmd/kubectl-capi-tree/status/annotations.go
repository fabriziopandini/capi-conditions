package status

import (
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// ShowObjectConditionsAnnotation documents that the presentation layer should show all the conditions for the object.
	ShowObjectConditionsAnnotation = "tree.cluster.x-k8s.io.io/show-conditions"

	// ObjectMetaNameAnnotation contains the meta name that should be used for the object in the presentation layer,
	// e.g. control plane for KCP.
	ObjectMetaNameAnnotation = "tree.cluster.x-k8s.io.io/meta-name"

	// VirtualObjectAnnotation documents that the object does not correspond to any physical object, but instead is
	// a virtual object introduced to provide a better representation of the cluster status, e.g. workers.
	VirtualObjectAnnotation = "tree.cluster.x-k8s.io.io/virtual-object"

	// GroupingObjectAnnotation documents that the child of this node will be grouped in case the ready condition
	// has the same Status, Severity and Reason.
	GroupingObjectAnnotation = "tree.cluster.x-k8s.io.io/grouping-object"

	// GroupObjectAnnotation documents that the object does not correspond to any physical object, but instead it is
	// a grouping of sibling nodes, e.g. a group of machines.
	GroupObjectAnnotation = "tree.cluster.x-k8s.io.io/group-object"

	// GroupItemsAnnotation contains the list of names for the objects included in a group object.
	GroupItemsAnnotation = "tree.cluster.x-k8s.io.io/group-items"

	// GroupItemsSeparator is the separator used in the GroupItemsAnnotation
	GroupItemsSeparator = ", "
)

func GetMetaName(obj controllerutil.Object) string {
	if val, ok := getAnnotation(obj, ObjectMetaNameAnnotation); ok {
		return val
	}
	return ""
}

func IsGroupingObject(obj controllerutil.Object) bool {
	if val, ok := getBoolAnnotation(obj, GroupingObjectAnnotation); ok {
		return val == true
	}
	return false
}

func IsGroupObject(obj controllerutil.Object) bool {
	if val, ok := getBoolAnnotation(obj, GroupObjectAnnotation); ok {
		return val == true
	}
	return false
}

func GetGroupItems(obj controllerutil.Object) string {
	if val, ok := getAnnotation(obj, GroupItemsAnnotation); ok {
		return val
	}
	return ""
}

func IsVirtualObject(obj controllerutil.Object) bool {
	if val, ok := getBoolAnnotation(obj, VirtualObjectAnnotation); ok {
		return val == true
	}
	return false
}

func IsShowConditionsObject(obj controllerutil.Object) bool {
	if val, ok := getBoolAnnotation(obj, ShowObjectConditionsAnnotation); ok {
		return val == true
	}
	return false
}

func getAnnotation(obj controllerutil.Object, annotation string) (string, bool) {
	val, ok := obj.GetAnnotations()[annotation]
	return val, ok
}

func getBoolAnnotation(obj controllerutil.Object, annotation string) (bool, bool) {
	val, ok := obj.GetAnnotations()[annotation]
	if ok {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			return boolVal, true
		}
	}
	return false, false
}

func addAnnotation(obj controllerutil.Object, annotation, value string) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[annotation] = value
	obj.SetAnnotations(annotations)
}
