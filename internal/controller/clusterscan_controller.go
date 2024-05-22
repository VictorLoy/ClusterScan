/*
Copyright 2024.

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
	"time"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	victortestv1 "github.com/VictorLoy/ClusterScan/api/v1"
)

// ClusterScanReconciler reconciles a ClusterScan object
type ClusterScanReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=victor.test,resources=clusterscans,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=victor.test,resources=clusterscans/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=victor.test,resources=clusterscans/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterScan object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile

func (r *ClusterScanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// TODO(user): your logic here
	var clusterScan victortestv1.ClusterScan
	if err := r.Get(ctx, req.NamespacedName, &clusterScan); err != nil {
		log.Error(err, "unable to fetch clusterScan")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if clusterScan.Spec.Schedule == "" {
		return r.ReconcileWithoutSchedule(ctx, &clusterScan)

	} else {
		return r.ReconcileWithSchedule(ctx, &clusterScan)
	}
}

func (r *ClusterScanReconciler) ReconcileWithSchedule(ctx context.Context, clusterScan *victortestv1.ClusterScan) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Hitting this S")

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterScan.Namespace,
			Name:      clusterScan.Name + "-cronjob",
		},

		Spec: batchv1.CronJobSpec{
			Schedule:    clusterScan.Spec.Schedule,
			JobTemplate: clusterScan.Spec.JobTemplate,
		},
	}

	if err := controllerutil.SetControllerReference(clusterScan, cronJob, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Check if the cronjob already exists
	found := &batchv1.CronJob{}
	err := r.Get(ctx, types.NamespacedName{Name: cronJob.Name, Namespace: cronJob.Namespace}, found)

	if err != nil && errors.IsNotFound(err) {
		// Create the Job
		if err := r.Create(ctx, cronJob); err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "unable to create cronJob")
			return ctrl.Result{}, err
		}
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// Update status with job details
	clusterScan.Status.LastRunTime = &metav1.Time{Time: time.Now()}

	// Watch for Job completion
	if len(found.Status.Active) > 0 {
		clusterScan.Status.CompletionStatus = "Active"
	} else {
		clusterScan.Status.CompletionStatus = "Not Active"
	}
	if err := r.Status().Update(ctx, clusterScan); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ClusterScanReconciler) ReconcileWithoutSchedule(ctx context.Context, clusterScan *victortestv1.ClusterScan) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Create Job Based on the sc
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterScan.Namespace,
			Name:      clusterScan.Name + "-job",
		},
		Spec: batchv1.JobSpec{
			Template: clusterScan.Spec.JobTemplate.Spec.Template,
		},
	}

	if err := controllerutil.SetControllerReference(clusterScan, job, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Check if the job already exists
	found := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, found)

	if err != nil && errors.IsNotFound(err) {
		// Create the Job
		if err := r.Create(ctx, job); err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "unable to create Job")
			return ctrl.Result{}, err
		}
	}

	clusterScan.Status.LastRunTime = &metav1.Time{Time: time.Now()}
	if found.Status.Succeeded > 0 {
		clusterScan.Status.CompletionStatus = "Completed"
	} else if found.Status.Failed > 0 {
		clusterScan.Status.CompletionStatus = "Failed"
	} else {
		clusterScan.Status.CompletionStatus = "Running"
	}
	if err := r.Status().Update(ctx, clusterScan); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterScanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&victortestv1.ClusterScan{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
