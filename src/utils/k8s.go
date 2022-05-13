package utils

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func GetNamespacedName(obj metav1.ObjectMeta) types.NamespacedName {
	return types.NamespacedName{
		Namespace: obj.Namespace,
		Name:      obj.Name,
	}
}

func GetNamespacedNameStr(obj metav1.ObjectMeta) string {
	return GetNamespacedName(obj).String()
}

func SplitNamespacedNameStr(namestring string, defaultNs string) (name, namespace string) {
	if namestring == "" {
		return "", ""
	}
	if !strings.Contains(namestring, "/") {
		return namestring, defaultNs
	}
	parts := strings.Split(namestring, "/")
	return parts[1], parts[0]
}

func AsNamespacedName(namestring string, defaultNs string) types.NamespacedName {
	name, ns := SplitNamespacedNameStr(namestring, defaultNs)
	return types.NamespacedName{Namespace: ns, Name: name}
}
