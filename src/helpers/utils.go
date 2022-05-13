package helpers

import (
	"context"
	"errors"

	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
)

//! NO USED
// LookupNodeCompartment returns the compartment OCID for the given nodeName.
func LookupNodeCompartment(k kubernetes.Interface, nodeName string) (string, error) {
	node, err := k.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if compartmentID, ok := node.ObjectMeta.Annotations["CompartmentIDAnnotation"]; ok {
		if compartmentID != "" {
			return compartmentID, nil
		}
	}
	return "", errors.New("CompartmentID annotation is not present")
}

// NodeInternalIP returns the nodes internal ip
// A node managed by the CCM will always have an internal ip
// since it's not possible to deploy an instance without a private ip.
func NodeInternalIP(node *api.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == api.NodeInternalIP {
			return addr.Address
		}
	}
	return ""
}

// RemoveDuplicatesFromList takes Slice and returns new Slice with no duplicate elements
// (e.g. if given list is {"a", "b", "a"}, function returns new slice with {"a", "b"}
func RemoveDuplicatesFromList(list []string) []string {
	return sets.NewString(list...).List()
}

// DeepEqualLists diffs two slices and returns bool if the slices are equal/not-equal.
// the duplicates and order of items in both lists is ignored.
func DeepEqualLists(listA, listB []string) bool {
	return sets.NewString(listA...).Equal(sets.NewString(listB...))
}
