package controllers

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"strings"
	"unicode"
)

type field struct {
	Name      string
	Resources []res
	options   tagOptions
	typ       reflect.Type
}

type res struct {
	Typ  string
	Path string
}

// tagOptions is the string following a comma in a struct field's "operator"
// tag, or the empty string. It does not include the leading comma.
type tagOptions string

// TypeFields extracts all operator relevant tags from the given reflect.Type
// Returns a map containing all fields referenced by its name
func TypeFields(t reflect.Type) []field {
	// Anonymous fields to explore at the current level and the next.
	var current []field
	next := []field{{typ: t}}

	// Count of queued names for current level and the next.
	var nextCount map[reflect.Type]int

	// Types already visited at an earlier level.
	visited := map[reflect.Type]bool{}

	// Fields found.
	var fields []field

	for len(next) > 0 {
		current, next = next, current[:0]
		nextCount = map[reflect.Type]int{}

		for _, f := range current {
			if visited[f.typ] {
				continue
			}
			visited[f.typ] = true

			// Scan f.typ for fields to include.
			for i := 0; i < f.typ.NumField(); i++ {
				sf := f.typ.Field(i)
				if sf.Anonymous {
					t := sf.Type
					if t.Kind() == reflect.Ptr {
						t = t.Elem()
					}
					if !sf.IsExported() && t.Kind() != reflect.Struct {
						// Ignore embedded fields of unexported non-struct types.
						continue
					}
					// Do not ignore embedded fields of unexported struct types
					// since they may have exported fields.
				} else if !sf.IsExported() {
					// Ignore unexported non-embedded fields.
					continue
				}
				tag := sf.Tag.Get("operator")
				if tag == "-" {
					continue
				}
				spec, opts := parseTag(tag)
				if !isValidTag(spec) {
					spec = ""
				}

				ft := sf.Type
				if ft.Name() == "" && ft.Kind() == reflect.Ptr {
					// Follow pointer.
					ft = ft.Elem()
				}

				// Record found field and index sequence.
				if spec != "" || ft.Kind() != reflect.Struct {
					if spec == "" {
						continue
					}
					resources := parseResources(spec)
					filed := field{
						Name:      sf.Name,
						Resources: resources,
						options:   opts,
						typ:       ft,
					}
					fields = append(fields, filed)
					continue
				}

				// Record new anonymous struct to explore in next round.
				nextCount[ft]++
				if nextCount[ft] == 1 {
					next = append(next, field{Name: ft.Name(), typ: ft})
				}
			}
		}
	}

	return fields
}

// parseResources splits a struct field's operator name into its resources
func parseResources(specs string) []res {
	var resources []res

	for _, spec := range strings.Split(specs, ";") {
		resources = append(resources, res{
			Typ:  parsResource(spec),
			Path: parsPath(spec),
		})
	}

	return resources
}

// parseTag splits a struct field's operator tag into its spec and
// comma-separated options.
func parseTag(tag string) (string, tagOptions) {
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx], tagOptions(tag[idx+1:])
	}
	return tag, tagOptions("")
}

func isValidTag(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", c):
			// Backslash and quote chars are reserved, but
			// otherwise any punctuation chars are allowed
			// in a tag name.
		case !unicode.IsLetter(c) && !unicode.IsDigit(c):
			return false
		}
	}
	return true
}

// parsResource splits a struct field's operator tag into its referenced kubernetes component
func parsResource(name string) string {
	if name == "" || !strings.ContainsRune(name, '.') {
		return ""
	}
	return strings.Split(name, ".")[0]
}

// parsPath splits a struct field's operator tag into its logic path of the watched kubernetes component
func parsPath(name string) string {
	if name == "" || !strings.ContainsRune(name, '.') {
		return ""
	}
	idx := strings.Index(name, ".")
	return name[idx+1:]
}

// getK8sResource returns an empty k8s resource based on the given input string
func getK8sResource(name string) metav1.Object {
	switch strings.ToLower(name) {
	case "deployment", "deploy":
		return &appsv1.Deployment{}
	case "daemonSet", "ds":
		return &appsv1.DaemonSet{}
	case "cronjob", "cj":
		return &batchv1.CronJob{}
	case "ingress", "ing":
		return &networkingv1.Ingress{}
	case "statefulset", "sts":
		return &appsv1.StatefulSet{}
	case "configmap", "cm":
		return &corev1.ConfigMap{}
	case "service", "svc":
		return &corev1.Service{}
	case "persistentvolumeclaims", "pvc":
		return &corev1.PersistentVolumeClaim{}
	case "serviceaccounts", "sa":
		return &corev1.ServiceAccount{}
	default:
		return nil
	}
}
