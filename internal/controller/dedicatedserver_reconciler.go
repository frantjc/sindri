package controller

import (
	"cmp"
	"context"
	"fmt"

	"github.com/frantjc/sindri/internal/api/v1alpha1"
	"github.com/frantjc/sindri/internal/logutil"
	xslices "github.com/frantjc/x/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// DedicatedServerReconciler reconciles a DedicatedServer object.
type DedicatedServerReconciler struct {
	client.Client
	record.EventRecorder
	Registry string
}

func (r *DedicatedServerReconciler) getDedicatedServer(ctx context.Context, key client.ObjectKey) (*v1alpha1.DedicatedServer, error) {
	ds := &v1alpha1.DedicatedServer{}

	if err := r.Get(ctx, key, ds); err != nil {
		return nil, err
	}

	return ds, nil
}

func needsHostPort(ds *v1alpha1.DedicatedServer) bool {
	return xslices.Some(ds.Spec.Ports, func(port v1alpha1.DedicatedServerPort, _ int) bool {
		return len(port.Protocols) > 1
	})
}

// +kubebuilder:rbac:groups=sindri.frantj.cc,resources=dedicatedservers,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=sindri.frantj.cc,resources=dedicatedservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sindri.frantj.cc,resources=dedicatedservers/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=appd,resources=deployments,verbs=get;list;watch;create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DedicatedServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		log = logutil.SloggerFrom(ctx)
	)

	log.Info("reconciling")

	ds, err := r.getDedicatedServer(ctx, req.NamespacedName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	ds.Status.Phase = v1alpha1.PhasePending

	if err := r.Status().Update(ctx, ds); err != nil {
		return ctrl.Result{}, err
	}

	var (
		containerPorts = []corev1.ContainerPort{}
		servicePorts   = []corev1.ServicePort{}
	)

	for _, port := range ds.Spec.Ports {
		for _, protocol := range port.Protocols {
			containerPorts = append(containerPorts, corev1.ContainerPort{
				ContainerPort: port.Port,
				Protocol:      protocol,
			})
			servicePorts = append(servicePorts, corev1.ServicePort{
				Name:       fmt.Sprintf("%s-%d-%s", ds.Name, port.Port, protocol),
				Port:       port.Port,
				TargetPort: intstr.FromInt32(port.Port),
				Protocol:   protocol,
			})
		}
	}

	var (
		hostNetwork = needsHostPort(ds)
		podSpec     = corev1.PodSpec{
			Volumes:     ds.Spec.Volumes,
			HostNetwork: hostNetwork,
			Containers: []corev1.Container{
				{
					Name:         ds.Name,
					Image:        fmt.Sprintf("%s/%d:%s", r.Registry, ds.Spec.AppID, ds.Spec.Branch),
					Ports:        containerPorts,
					VolumeMounts: ds.Spec.VolumeMounts,
					Resources:    ds.Spec.Resources,
				},
			},
		}
		podLabels = map[string]string{
			"dedicatedserver.sindri.frantj.cc/name": ds.Name,
		}
		deploymentSpec = appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: podLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: podSpec,
			},
		}
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ds.Name,
				Namespace: ds.Namespace,
				Labels:    podLabels,
			},
			Spec: deploymentSpec,
		}
	)

	if err := controllerutil.SetControllerReference(ds, deployment, r.Scheme()); err != nil {
		r.Event(ds, corev1.EventTypeWarning, "ReconcileDeployment", err.Error())
		ds.Status.Phase = v1alpha1.PhaseFailed
		return ctrl.Result{}, r.Status().Update(ctx, ds)
	}

	if _, err = controllerutil.CreateOrUpdate(ctx, r, deployment, func() error {
		deployment.Spec = deploymentSpec
		return controllerutil.SetControllerReference(ds, deployment, r.Scheme())
	}); err != nil {
		r.Event(ds, corev1.EventTypeWarning, "ReconcileDeployment", err.Error())
		ds.Status.Phase = v1alpha1.PhaseFailed
		return ctrl.Result{}, r.Status().Update(ctx, ds)
	}

	if !hostNetwork {
		var (
			svcSpec = corev1.ServiceSpec{
				Selector: podLabels,
				Ports:    servicePorts,
			}
			svc = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ds.Name,
					Namespace: ds.Namespace,
				},
				Spec: svcSpec,
			}
		)

		if err := controllerutil.SetControllerReference(ds, svc, r.Scheme()); err != nil {
			r.Event(ds, corev1.EventTypeWarning, "ReconcileService", err.Error())
			ds.Status.Phase = v1alpha1.PhaseFailed
			return ctrl.Result{}, r.Status().Update(ctx, ds)
		}

		if res, err := controllerutil.CreateOrUpdate(ctx, r, svc, func() error {
			svc.Spec = svcSpec
			return controllerutil.SetControllerReference(ds, svc, r.Scheme())
		}); err != nil {
			r.Event(ds, corev1.EventTypeWarning, "ReconcileService", err.Error())
			ds.Status.Phase = v1alpha1.PhaseFailed
			return ctrl.Result{}, r.Status().Update(ctx, ds)
		} else if res == controllerutil.OperationResultCreated {
			// If it was just created, wait on it to be reconciled and trigger another reconciliation for us.
			return ctrl.Result{}, nil
		}

		for _, ing := range svc.Status.LoadBalancer.Ingress {
			ds.Status.IP = cmp.Or(ing.Hostname, ing.IP)

			if ds.Status.IP != "" {
				break
			}
		}
	} else {
		pods := &corev1.PodList{}

		if err := r.List(ctx, pods, client.InNamespace(ds.Namespace), client.MatchingLabels(podLabels)); err != nil {
			r.Event(ds, corev1.EventTypeWarning, "ReconcileDeployment", err.Error())
			ds.Status.Phase = v1alpha1.PhaseFailed
			return ctrl.Result{}, r.Status().Update(ctx, ds)
		}

		for _, pod := range pods.Items {
			ds.Status.IP = pod.Status.HostIP

			if ds.Status.IP != "" {
				break
			}
		}
	}

	ds.Status.Phase = v1alpha1.PhaseReady

	if err := r.Status().Update(ctx, ds); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DedicatedServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()
	r.EventRecorder = mgr.GetEventRecorderFor("sindri")

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DedicatedServer{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r); err != nil {
		return err
	}

	return nil
}
