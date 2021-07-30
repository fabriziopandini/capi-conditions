package status

import (
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func GetReadyCondition(obj controllerutil.Object) *clusterv1.Condition {
	getter := objToGetter(obj)
	if getter == nil {
		return nil
	}
	return conditions.Get(getter, clusterv1.ReadyCondition)
}

func GetOtherConditions(obj controllerutil.Object) []*clusterv1.Condition {
	getter := objToGetter(obj)
	if getter == nil {
		return nil
	}
	var conditions []*clusterv1.Condition
	for _, c := range getter.GetConditions() {
		c := c
		if c.Type != clusterv1.ReadyCondition {
			conditions = append(conditions, &c)
		}
	}
	sort.Slice(conditions, func(i, j int) bool {
		return conditions[i].Type < conditions[j].Type
	})
	return conditions
}

func setReadyCondition(obj controllerutil.Object, ready *clusterv1.Condition) {
	setter := objToSetter(obj)
	if setter == nil {
		return
	}
	conditions.Set(setter, ready)
}

func objToGetter(obj controllerutil.Object) conditions.Getter {
	if getter, ok := obj.(conditions.Getter); ok {
		return getter
	}

	objUnstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil
	}
	getter := conditions.UnstructuredGetter(objUnstructured)
	return getter
}

func objToSetter(obj controllerutil.Object) conditions.Setter {
	if setter, ok := obj.(conditions.Setter); ok {
		return setter
	}

	objUnstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil
	}
	setter := conditions.UnstructuredSetter(objUnstructured)
	return setter
}

// TODO: consider if to use unstructured & if we can make type meta more expressive (e.g. set API version, add GVK to uid or cloning an empty object);
//  as of today this is not because it impacts sorting
// TODO: split name and UID
func virtualObject(namespace, name string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "",
			Kind:       name,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Annotations: map[string]string{
				VirtualObjectAnnotation: "True",
			},
			UID: types.UID(fmt.Sprintf("%s, %s/%s", "", namespace, name)),
		},
	}
}
