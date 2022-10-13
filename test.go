package main

import (
	"git.pc-ziegert.de/operators/satisfactory/api/v1alpha1"
	"git.pc-ziegert.de/operators/satisfactory/controllers"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"strings"
)

type field struct {
	name     string
	resource metav1.Object
	path     string
	options  tagOptions
	typ      reflect.Type
}

// tagOptions is the string following a comma in a struct field's "operator"
// tag, or the empty string. It does not include the leading comma.
type tagOptions string

func main() {
	fs := v1alpha1.FactoryServer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "FactoryServer",
			APIVersion: "satisfactory.pc-ziegert.de/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "factoryserver-sample",
			Namespace: "satisfactory",
		},
		Spec: v1alpha1.FactoryServerSpec{
			AutosaveInterval:      300,
			Autopause:             true,
			AutosaveNum:           5,
			AutosaveOnDisconnect:  true,
			CrashReport:           false,
			Debug:                 false,
			DisableSeasonalEvents: false,
			Image:                 "wolveix/satisfactory-server:latest",
			Maxplayers:            4,
			Networkquality:        3,
			PortBeacon:            15000,
			PortGame:              7777,
			PortQuery:             15777,
			ServiceNodePortBeacon: 15000,
			ServiceNodePortGame:   7777,
			ServiceNodePortQuery:  15777,
			ServiceType:           "ClusterIP",
			Size:                  1,
			SkipUpdate:            false,
			Beta:                  false,
			StorageClass:          "nfs-ssd01",
			StorageRequests:       "50Gi",
		},
		Status: v1alpha1.FactoryServerStatus{},
	}
	fields := controllers.TypeFields(reflect.TypeOf(fs.Spec))
	sts := *getStatefulSet(&fs)
	sts2 := *getStatefulSet(&fs)
	sts2.Spec.Replicas = &[]int32{4}[0]
	sts2.Spec.Template.Spec.Containers[0].Image = "2"
	sts2.Spec.VolumeClaimTemplates[0].Spec.StorageClassName = &[]string{"asdasdasd"}[0]
	sts2.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests["storage"] = resource.MustParse("10Gi")
	sts2.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = 5
	sts2.Spec.Template.Spec.Containers[0].Ports[1].ContainerPort = 6
	sts2.Spec.Template.Spec.Containers[0].Ports[2].ContainerPort = 7
	for _, filed := range fields {
		for _, resource := range filed.Resources {
			switch strings.ToLower(resource.Typ) {
			case "deployment", "deploy":
				continue
			case "daemonSet", "ds":
				continue
			case "cronjob", "cj":
				continue
			case "ingress", "ing":
				continue
			case "statefulset", "sts":
				println("compare: " + resource.Typ + " : " + resource.Path)
				if test(&sts, &sts2, resource.Path) {
					println("adjusted")
				} else {
					println("matched")
				}
				print("")
			case "configmap", "cm":
				continue
			case "service", "svc":
				continue
			case "persistentvolumeclaims", "pvc":
				continue
			case "serviceaccounts", "sa":
				continue
			default:
				continue
			}
		}
	}
}

func test(source interface{}, target interface{}, fieldPath string) bool {

	// if target is not pointer, then immediately return
	// modifying struct's field requires addressable object
	sAddrValue := reflect.ValueOf(source)
	tAddrValue := reflect.ValueOf(target)
	if sAddrValue.Kind() != reflect.Ptr || tAddrValue.Kind() != reflect.Ptr {
		panic("Fuuu!!!")
	}

	sValue := sAddrValue.Elem()
	tValue := tAddrValue.Elem()
	if !sValue.IsValid() || !tValue.IsValid() {
		panic("Fuuu!!!")
	}

	// If the field/struct is passed by pointer, then first dereference it to get the
	// underlying value (the pointer must not be pointing to a nil value).
	if sValue.Type().Kind() == reflect.Ptr && !sValue.IsNil() ||
		tValue.Type().Kind() == reflect.Ptr && !tValue.IsNil() {
		sValue = sValue.Elem()
		tValue = tValue.Elem()
		if !sValue.IsValid() || !tValue.IsValid() {
			panic("Fuuu!!!")
		}
	}

	// split filedPath into individual pieces
	// walk through given path
	for _, fieldName := range strings.Split(fieldPath, ".") {
		if sValue.Type().Kind() == reflect.Ptr && !sValue.IsNil() ||
			tValue.Type().Kind() == reflect.Ptr && !tValue.IsNil() {
			sValue = sValue.Elem()
			tValue = tValue.Elem()
			if !sValue.IsValid() || !tValue.IsValid() {
				panic("Fuuu!!!")
			}
		}

		// if given field is an array
		if strings.ContainsAny(fieldName, "[]") {
			idx := strings.Index(fieldName, "[")
			listElementName := fieldName[idx+1 : len(fieldName)-1]
			fieldName = fieldName[:idx]
			sValue = sValue.FieldByName(fieldName)
			tValue = tValue.FieldByName(fieldName)

			if strings.ContainsRune(listElementName, '=') {
				idx = strings.Index(listElementName, "=")
				keys := listElementName[:idx]
				value := listElementName[idx+1:]
				for i := 0; i < sValue.Len(); i++ {
					sValueIndex := sValue.Index(i)
					for _, key := range strings.Split(keys, ":") {
						sValueIndex = sValueIndex.FieldByName(key)
						if sValueIndex.Type().Kind() == reflect.Ptr && !sValueIndex.IsNil() {
							sValueIndex = sValueIndex.Elem()
						}
					}
					if sValueIndex.String() == value {
						sValue = sValue.Index(i)
						break
					}
				}
				for i := 0; i < tValue.Len(); i++ {
					tValueIndex := tValue.Index(i)
					for _, key := range strings.Split(keys, ":") {
						tValueIndex = tValueIndex.FieldByName(key)
						if tValueIndex.Type().Kind() == reflect.Ptr && !tValueIndex.IsNil() {
							tValueIndex = tValueIndex.Elem()
						}
					}
					if tValueIndex.String() == value {
						tValue = tValue.Index(i)
						break
					}
				}
				continue
			}

			//search for next element by name
			for i := 0; i < tValue.Len(); i++ {
				if tValue.Index(i).FieldByName("Name").String() == listElementName {
					tValue = tValue.Index(i)
					break
				}
			}
			for i := 0; i < sValue.Len(); i++ {
				if sValue.Index(i).FieldByName("Name").String() == listElementName {
					sValue = sValue.Index(i)
					break
				}
			}
			continue
		}

		// if given field is a map
		if strings.ContainsAny(fieldName, "{}") {
			idx := strings.Index(fieldName, "{")
			mapElementName := fieldName[idx+1 : len(fieldName)-1]
			fieldName = fieldName[:idx]
			sValue = sValue.FieldByName(fieldName)
			tValue = tValue.FieldByName(fieldName)

			//search for next element by name
			for _, key := range sValue.MapKeys() {
				keyPtr := key
				if keyPtr.Type().Kind() == reflect.Ptr && !keyPtr.IsNil() {
					keyPtr = keyPtr.Elem()
				}
				if keyPtr.String() == mapElementName {
					sValue = sValue.MapIndex(key)
					break
				}
			}
			for _, key := range tValue.MapKeys() {
				keyPtr := key
				if keyPtr.Type().Kind() == reflect.Ptr && !keyPtr.IsNil() {
					keyPtr = keyPtr.Elem()
				}
				if keyPtr.String() == mapElementName {
					tValue = tValue.MapIndex(key)
					break
				}
			}
			continue
		}

		sValue = sValue.FieldByName(fieldName)
		tValue = tValue.FieldByName(fieldName)
	}

	if sValue.Type().Kind() == reflect.Ptr && !sValue.IsNil() ||
		tValue.Type().Kind() == reflect.Ptr && !tValue.IsNil() {
		sValue = sValue.Elem()
		tValue = tValue.Elem()
		if !sValue.IsValid() || !tValue.IsValid() {
			panic("Fuuu!!!")
		}
	}

	if reflect.DeepEqual(sValue.Interface(), tValue.Interface()) {
		return false
	}
	//check if tValue is a correct Value
	if !tValue.IsValid() {
		panic("Fuuu!!!")
	}

	// check if we can set this value
	// ToDo: Fails if struct needs to be updated
	// Spec.VolumeClaimTemplates.Spec.Resources.Requests{storage}
	if !tValue.CanSet() {
		return false
	}

	// set actual value
	tValue.Set(sValue)
	return true
}

// getStatefulSet returns a factoryServer StatefulSet object
func getStatefulSet(f *v1alpha1.FactoryServer) *appsv1.StatefulSet {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "getStatefulSetName(f)",
			Namespace: f.Namespace,
			Labels: map[string]string{
				"getLabels(f)": "getLabels(f)",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &f.Spec.Size,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"getLabels(f)": "getLabels(f)",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"getLabels(f)": "getLabels(f)",
					},
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:    &[]int64{0}[0],
						RunAsGroup:   &[]int64{0}[0],
						RunAsNonRoot: &[]bool{false}[0],
						FSGroup:      &[]int64{0}[0],
					},
					Containers: []corev1.Container{{
						Name:  "getServiceName(f)",
						Image: f.Spec.Image,
						Ports: []corev1.ContainerPort{
							{
								Name:          "query",
								ContainerPort: f.Spec.PortQuery,
								Protocol:      "UDP",
							},
							{
								Name:          "beacon",
								ContainerPort: f.Spec.PortBeacon,
								Protocol:      "UDP",
							},
							{
								Name:          "game",
								ContainerPort: f.Spec.PortGame,
								Protocol:      "UDP",
							},
						},
						EnvFrom: []corev1.EnvFromSource{
							{
								ConfigMapRef: &corev1.ConfigMapEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "getConfigMapName(f)",
									},
									Optional: &[]bool{false}[0],
								},
							},
						},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("4"),
								corev1.ResourceMemory: resource.MustParse("16Gi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "getPersistentVolumeClaimName(f)",
								MountPath: "/config",
							},
						},
						LivenessProbe:  nil,
						ReadinessProbe: nil,
						StartupProbe:   nil,
						SecurityContext: &corev1.SecurityContext{
							//Capabilities: &corev1.Capabilities{
							//	Add:  []corev1.Capability{"CAP_CHOWN", "CAP_SETGID", "CAP_SETUID"},
							//	Drop: []corev1.Capability{"ALL"},
							//},
							Privileged:               &[]bool{false}[0],
							RunAsUser:                &[]int64{0}[0],
							RunAsGroup:               &[]int64{0}[0],
							RunAsNonRoot:             &[]bool{false}[0],
							AllowPrivilegeEscalation: &[]bool{false}[0],
						},
					}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "getPersistentVolumeClaimName(f)",
					Namespace: f.Namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(f.Spec.StorageRequests),
						},
					},
				},
			}},
			PersistentVolumeClaimRetentionPolicy: &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
				WhenDeleted: appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
				WhenScaled:  appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
			},
		},
	}

	if len(f.Spec.StorageClass) > 0 {
		sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName = &f.Spec.StorageClass
	}
	return sts
}
