package status

import (
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type objectTreeOptions struct {
	// ShowOtherConditions is a list of comma separated kind or kind/name for which we should add   the ShowObjectConditionsAnnotation
	// to signal to the presentation layer to show all the conditions for the objects.
	ShowOtherConditions string

	// DisableNoEcho disables hiding objects if the object's ready condition has the
	// same Status, Severity and Reason of the parent's object ready condition (it is an echo)
	DisableNoEcho bool

	// DisableGroupObjects disables grouping sibling objects in case the ready condition
	// has the same Status, Severity and Reason
	DisableGroupObjects bool

	// DebugFilter is a list of kind or kind/name for which we should add ShowObjectConditionsAnnotation.
	DebugFilter string
}

type ObjectTree struct {
	options   objectTreeOptions
	items     map[types.UID]controllerutil.Object
	ownership map[types.UID]map[types.UID]bool
}

func newObjectTree(options objectTreeOptions) *ObjectTree {
	return &ObjectTree{
		options:   options,
		items:     make(map[types.UID]controllerutil.Object),
		ownership: make(map[types.UID]map[types.UID]bool),
	}
}

func (od ObjectTree) add(parent, obj controllerutil.Object, opts ...AddObjectOption) {
	addOpts := &AddObjectOptions{}
	addOpts.ApplyOptions(opts)

	objReady := GetReadyCondition(obj)
	parentReady := GetReadyCondition(parent)

	// If it is requested to show all the conditions for the object, add
	// the ShowObjectConditionsAnnotation to signal this to the presentation layer.
	if isObjDebug(obj, od.options.ShowOtherConditions) {
		addAnnotation(obj, ShowObjectConditionsAnnotation, "True")
	}

	// If the object should be hidden if the object's ready condition is true ot it has the
	// same Status, Severity and Reason of the parent's object ready condition (it is an echo),
	// return early.
	if addOpts.NoEcho && !od.options.DisableNoEcho {
		if (objReady != nil && objReady.Status == corev1.ConditionTrue) || hasSameReadyStatusSeverityAndReason(parentReady, objReady) {
			return
		}
	}

	// If it is requested to use a meta name for the object in the presentation layer, add
	// the ObjectMetaNameAnnotation to signal this to the presentation layer.
	if addOpts.MetaName != "" {
		addAnnotation(obj, ObjectMetaNameAnnotation, addOpts.MetaName)
	}

	// If it is requested that this object and its sibling should be grouped in case the ready condition
	// has the same Status, Severity and Reason, process all the sibling nodes.
	if IsGroupingObject(parent) {
		siblings := od.GetObjectsByParent(parent.GetUID())

		for i := range siblings {
			s := siblings[i]
			sReady := GetReadyCondition(s)

			// If the object's ready condition has a different Status, Severity and Reason than the sibling object,
			// move on (they should not be grouped).
			if !hasSameReadyStatusSeverityAndReason(objReady, sReady) {
				continue
			}

			// If the sibling node is already a group object, upgrade it with the current object.
			if IsGroupObject(s) {
				updateGroupNode(s, sReady, obj, objReady)
				return
			}

			// Otherwise the object and the current sibling should be merged in a group.

			// Create virtual object for the group and add it to the object tree.
			groupNode := createGroupNode(s, sReady, obj, objReady)
			od.addInner(parent, groupNode)

			// Remove the current sibling (now merged in the group).
			od.remove(parent, s)
			return
		}
	}

	// If it is requested that the child of this node should be grouped in case the ready condition
	// has the same Status, Severity and Reason, add the GroupingObjectAnnotation to signal
	// this to the presentation layer.
	if addOpts.GroupingObject && !od.options.DisableGroupObjects {
		addAnnotation(obj, GroupingObjectAnnotation, "True")
	}

	// Add the object to the object tree.
	od.addInner(parent, obj)
}

func (od ObjectTree) remove(parent controllerutil.Object, s controllerutil.Object) {
	delete(od.items, s.GetUID())
	delete(od.ownership[parent.GetUID()], s.GetUID())
}

func (od ObjectTree) addInner(parent controllerutil.Object, obj controllerutil.Object) {
	od.items[obj.GetUID()] = obj
	if od.ownership[parent.GetUID()] == nil {
		od.ownership[parent.GetUID()] = make(map[types.UID]bool)
	}
	od.ownership[parent.GetUID()][obj.GetUID()] = true
}

func (od ObjectTree) GetObject(id types.UID) controllerutil.Object { return od.items[id] }

func (od ObjectTree) GetObjectsByParent(id types.UID) []controllerutil.Object {
	var out []controllerutil.Object
	for k := range od.ownership[id] {
		out = append(out, od.GetObject(k))
	}
	return out
}

func hasSameReadyStatusSeverityAndReason(a, b *clusterv1.Condition) bool {
	if a == nil && b == nil {
		return true
	}
	if (a == nil) != (b == nil) {
		return false
	}

	return a.Status == b.Status &&
		a.Severity == b.Severity &&
		a.Reason == b.Reason
}

func createGroupNode(s controllerutil.Object, sReady *clusterv1.Condition, obj controllerutil.Object, objReady *clusterv1.Condition) *clusterv1.Cluster {
	// TODO: pass Kind to the tree object
	// Create a new group node and add the GroupObjectAnnotation to signal
	// this to the presentation layer.
	// NB. The group nodes gets a unique ID to avoid conflicts.
	groupNode := virtualObject(obj.GetNamespace(), readyStatusSeverityAndReasonUID(obj))
	addAnnotation(groupNode, GroupObjectAnnotation, "True")

	// Update the list of items included in the group and store it in the GroupItemsAnnotation.
	items := []string{obj.GetName(), s.GetName()}
	sort.Strings(items)
	addAnnotation(groupNode, GroupItemsAnnotation, strings.Join(items, GroupItemsSeparator))

	// Update the group's ready condition.
	if objReady != nil {
		objReady.LastTransitionTime = minLastTransitionTime(objReady, sReady)
		objReady.Message = ""
		setReadyCondition(groupNode, objReady)
	}
	return groupNode
}

func readyStatusSeverityAndReasonUID(obj controllerutil.Object) string {
	ready := GetReadyCondition(obj)
	if ready == nil {
		return fmt.Sprintf("zzz_%s", util.RandomString(6))
	}
	return fmt.Sprintf("zz_%s_%s_%s_%s", ready.Status, ready.Severity, ready.Reason, util.RandomString(6))
}

func minLastTransitionTime(a, b *clusterv1.Condition) metav1.Time {
	if a == nil && b == nil {
		return metav1.Time{}
	}
	if (a != nil) && (b == nil) {
		return a.LastTransitionTime
	}
	if (a == nil) && (b != nil) {
		return a.LastTransitionTime
	}
	if a.LastTransitionTime.Time.After(b.LastTransitionTime.Time) {
		return a.LastTransitionTime
	}
	return b.LastTransitionTime
}

func updateGroupNode(s controllerutil.Object, sReady *clusterv1.Condition, obj controllerutil.Object, objReady *clusterv1.Condition) {
	// Update the list of items included in the group and store it in the GroupItemsAnnotation.
	items := strings.Split(GetGroupItems(s), GroupItemsSeparator)
	items = append(items, obj.GetName())
	sort.Strings(items)
	addAnnotation(s, GroupItemsAnnotation, strings.Join(items, GroupItemsSeparator))

	// Update the group's ready condition.
	if sReady != nil {
		sReady.LastTransitionTime = minLastTransitionTime(objReady, sReady)
		sReady.Message = ""
		setReadyCondition(s, sReady)
	}
}

func isObjDebug(obj controllerutil.Object, debugFilter string) bool {
	if debugFilter == "" {
		return false
	}
	for _, filter := range strings.Split(debugFilter, ",") {
		if filter == "" {
			continue
		}
		if strings.ToLower(filter) == "all" {
			return true
		}
		kn := strings.Split(filter, "/")
		if len(kn) == 2 {
			if obj.GetObjectKind().GroupVersionKind().Kind == kn[0] && obj.GetName() == kn[1] {
				return true
			}
			continue
		}
		if obj.GetObjectKind().GroupVersionKind().Kind == kn[0] {
			return true
		}
	}
	return false
}
