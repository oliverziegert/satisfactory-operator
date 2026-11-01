/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
	"github.com/oliverziegert/satisfactory-operator/internal/backup"
	"github.com/oliverziegert/satisfactory-operator/internal/conditions"
	satisfactory "github.com/oliverziegert/satisfactory-operator/pkg/satisfactory"
)

// reachableServer fetches the referenced SatisfactoryServer and checks ServerReachable,
// without calling the game API. Failures that map to a Ready condition reason are returned
// as *authHelperError.
func (r *SaveGameReconciler) reachableServer(ctx context.Context, serverRef, namespace string) (*satisfactoryv1alpha1.SatisfactoryServer, error) {
	server := &satisfactoryv1alpha1.SatisfactoryServer{}
	if err := r.Get(ctx, types.NamespacedName{Name: serverRef, Namespace: namespace}, server); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &authHelperError{
				Reason:  reasonServerNotFound,
				Message: fmt.Sprintf("SatisfactoryServer %q not found in namespace %q", serverRef, namespace),
			}
		}
		return nil, fmt.Errorf("get SatisfactoryServer %s: %w", serverRef, err)
	}
	if !conditions.IsTrueSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeServerReachable) {
		return nil, &authHelperError{
			Reason:  reasonServerNotReachable,
			Message: fmt.Sprintf("SatisfactoryServer %q is not reachable", serverRef),
		}
	}
	return server, nil
}

// authenticatedServer returns the reachable server after making sure its cached auth token
// (the <server>-auth-token Secret the backup Job mounts) is valid. Failures that map to a
// Ready condition reason (ServerNotFound, ServerNotReachable, ServerNotAuthenticated) are
// returned as *authHelperError.
func (r *SaveGameReconciler) authenticatedServer(ctx context.Context, serverRef, namespace string) (*satisfactoryv1alpha1.SatisfactoryServer, error) {
	server, err := r.reachableServer(ctx, serverRef, namespace)
	if err != nil {
		return nil, err
	}
	if server.Spec.AdminPasswordSecretRef == nil {
		return nil, &authHelperError{
			Reason:  reasonServerNotAuthenticated,
			Message: fmt.Sprintf("SatisfactoryServer %q has no adminPasswordSecretRef", serverRef),
		}
	}

	c := satisfactory.NewClient(r.gameAPIURL(server), &tls.Config{InsecureSkipVerify: true}) //nolint:gosec // self-signed cert expected

	// Reuse the server controller's token cache + login flow; it records the outcome
	// as the Authenticated condition on this in-memory copy (never persisted here).
	sr := &SatisfactoryServerReconciler{Client: r.Client, Scheme: r.Scheme}
	sr.authenticateClient(ctx, server, c)
	if !conditions.IsTrueSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeAuthenticated) {
		msg := "authentication failed"
		if cond := meta.FindStatusCondition(server.Status.Conditions, satisfactoryv1alpha1.SatisfactoryServerConditionTypeAuthenticated); cond != nil {
			msg = cond.Message
		}
		return nil, &authHelperError{Reason: reasonServerNotAuthenticated, Message: msg}
	}
	return server, nil
}

func (r *SaveGameReconciler) gameAPIURL(server *satisfactoryv1alpha1.SatisfactoryServer) string {
	if r.GameAPIURL != nil {
		return r.GameAPIURL(server)
	}
	return gameAPIBaseURL(server)
}

// nextBackup returns when the next backup is due; ok is false once a one-shot backup succeeded.
// status.NextBackupTime is set when a Job is created, so a failed run waits for the next cron
// tick instead of starting another save right away. lastRun is the newest backup Job's run time.
func nextBackup(sched cron.Schedule, status satisfactoryv1alpha1.SaveGameStatus, lastRun *time.Time, now time.Time) (time.Time, bool) {
	if sched == nil {
		switch {
		case status.LastBackupTime != nil:
			return time.Time{}, false
		case lastRun != nil:
			return lastRun.Add(saveGameRetryInterval), true
		}
		return now, true
	}
	switch {
	case status.NextBackupTime != nil:
		// The min picks up a schedule that changed to run sooner.
		next := sched.Next(now)
		if status.NextBackupTime.Time.Before(next) {
			next = status.NextBackupTime.Time
		}
		return next, true
	case status.LastBackupTime != nil:
		return sched.Next(status.LastBackupTime.Time), true
	}
	return now, true
}

// buildBackupJob returns the Job that runs one backup of sg as saveName.
func buildBackupJob(
	sg *satisfactoryv1alpha1.SaveGame,
	scheme *runtime.Scheme,
	image, serverName, serverURL, saveName string,
	at time.Time,
) (*batchv1.Job, error) {
	cfg, err := json.Marshal(backup.Resolve(sg.Spec))
	if err != nil {
		return nil, fmt.Errorf("marshal destinations: %w", err)
	}
	// The job-name label on the pod must stay a valid 63-character label value.
	suffix := fmt.Sprintf("-%d", at.Unix())
	prefix := sg.Name
	if len(prefix) > 63-len(suffix) {
		prefix = strings.TrimRight(prefix[:63-len(suffix)], "-.")
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        prefix + suffix,
			Namespace:   sg.Namespace,
			Labels:      map[string]string{"app.kubernetes.io/managed-by": "satisfactory-operator"},
			Annotations: map[string]string{backupTimeAnnotation: at.UTC().Format(time.RFC3339)},
		},
		Spec: batchv1.JobSpec{
			// A retry would trigger a second save; the next schedule tick tries again instead.
			BackoffLimit:            ptr.To[int32](0),
			ActiveDeadlineSeconds:   ptr.To[int64](1800),
			TTLSecondsAfterFinished: ptr.To[int32](3600),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:                corev1.RestartPolicyNever,
					AutomountServiceAccountToken: ptr.To(false),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot:   ptr.To(true),
						SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
						// The distroless nonroot group, so the Job can write to fresh pvc destinations.
						FSGroup:             ptr.To[int64](65532),
						FSGroupChangePolicy: ptr.To(corev1.FSGroupChangeOnRootMismatch),
					},
					Containers: []corev1.Container{{
						Name:            backupContainerName,
						Image:           image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         []string{"/manager"},
						Args: []string{"backup",
							"--server-url=" + serverURL,
							"--save-name=" + saveName,
							"--session-prefix=" + sg.Spec.SaveName,
							"--config=" + string(cfg),
							"--token-file=/var/run/satisfactory/token",
						},
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: ptr.To(false),
							ReadOnlyRootFilesystem:   ptr.To(true),
							RunAsNonRoot:             ptr.To(true),
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
							Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("256Mi")},
						},
						VolumeMounts: []corev1.VolumeMount{
							// The save is downloaded to os.TempDir().
							{Name: "scratch", MountPath: "/tmp"},
							{Name: authTokenKey, MountPath: "/var/run/satisfactory", ReadOnly: true},
						},
					}},
					Volumes: []corev1.Volume{
						{Name: "scratch", VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: ptr.To(resource.MustParse("2Gi"))},
						}},
						{Name: authTokenKey, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
							SecretName: authSecretNameFor(serverName),
							Items:      []corev1.KeyToPath{{Key: authTokenKey, Path: authTokenKey}},
						}}},
					},
				},
			},
		},
	}
	pod := &job.Spec.Template.Spec
	for _, d := range sg.Spec.Destinations {
		if d.PVC == nil {
			continue
		}
		vol := "dest-" + d.Name
		pod.Volumes = append(pod.Volumes, corev1.Volume{Name: vol, VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: d.PVC.ClaimName},
		}})
		pod.Containers[0].VolumeMounts = append(pod.Containers[0].VolumeMounts,
			corev1.VolumeMount{Name: vol, MountPath: path.Join(backup.PVCMountRoot, d.Name)})
	}
	if err := controllerutil.SetControllerReference(sg, job, scheme); err != nil {
		return nil, fmt.Errorf("set owner reference: %w", err)
	}
	return job, nil
}

// jobRunTime returns the run time stored on a backup Job, or nil if it has none.
func jobRunTime(job *batchv1.Job) *time.Time {
	if job == nil {
		return nil
	}
	t, err := time.Parse(time.RFC3339, job.Annotations[backupTimeAnnotation])
	if err != nil {
		return nil
	}
	return &t
}

// jobFinished reports whether the Job completed or failed, and whether it completed.
func jobFinished(job *batchv1.Job) (finished, succeeded bool) {
	for _, c := range job.Status.Conditions {
		if c.Status != corev1.ConditionTrue {
			continue
		}
		switch c.Type {
		case batchv1.JobComplete:
			return true, true
		case batchv1.JobFailed:
			return true, false
		}
	}
	return false, false
}

// jobFailureMessage returns the Job's Failed condition message, if any.
func jobFailureMessage(job *batchv1.Job) string {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return c.Message
		}
	}
	return ""
}

// applyBackupResult copies a Job result into sg.Status and returns the Ready condition to set.
func applyBackupResult(sg *satisfactoryv1alpha1.SaveGame, res backup.Result, at time.Time) (
	phase satisfactoryv1alpha1.SaveGamePhase, status metav1.ConditionStatus, reason, message string,
) {
	serverName := ""
	for _, d := range backup.Resolve(sg.Spec) {
		if d.Server != nil {
			serverName = d.Name
		}
	}
	prev := map[string]satisfactoryv1alpha1.SaveGameDestinationStatus{}
	for _, d := range sg.Status.Destinations {
		prev[d.Name] = d
	}
	runTime := metav1.Time{Time: at}
	size := res.Size
	sg.Status.Destinations = nil
	sg.Status.RetainedSaveCount = 0
	var failed []string
	for _, d := range res.Destinations {
		ds := prev[d.Name]
		ds.Name = d.Name
		if d.OK {
			ds.LastSuccessTime, ds.LastSize, ds.RetainedCount, ds.LastError = &runTime, &size, d.Retained, ""
		} else {
			ds.LastError = d.Error
			failed = append(failed, d.Name+": "+d.Error)
		}
		if d.Name == serverName {
			sg.Status.RetainedSaveCount = ds.RetainedCount
		}
		sg.Status.Destinations = append(sg.Status.Destinations, ds)
	}

	if !res.Saved {
		return satisfactoryv1alpha1.SaveGamePhaseFailed, metav1.ConditionFalse, "SaveFailed", res.Error
	}
	sg.Status.LastBackupTime = &runTime
	sg.Status.BackupSize = &size
	if len(failed) > 0 {
		return satisfactoryv1alpha1.SaveGamePhaseFailed, metav1.ConditionFalse, "DestinationFailed",
			"Destinations failed: " + strings.Join(failed, "; ")
	}
	phase = satisfactoryv1alpha1.SaveGamePhaseActive
	if sg.Spec.Schedule == "" {
		phase = satisfactoryv1alpha1.SaveGamePhaseCompleted
	}
	return phase, metav1.ConditionTrue, "Reconciled",
		fmt.Sprintf("Backup %s written to %d destination(s)", res.SaveName, len(res.Destinations))
}
