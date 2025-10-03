package controller

import (
	"cmp"
	"context"
	"fmt"

	"github.com/frantjc/sindri/internal/api/v1alpha1"
	"github.com/frantjc/sindri/internal/logutil"
	xslices "github.com/frantjc/x/slices"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// DedicatedServerReconciler reconciles a DedicatedServer object.
type DedicatedServerReconciler struct {
	client.Client
	record.EventRecorder
	BoilerHost string
}

func (r *DedicatedServerReconciler) getDedicatedServer(ctx context.Context, key client.ObjectKey) (*v1alpha1.DedicatedServer, error) {
	ds := &v1alpha1.DedicatedServer{}

	if err := r.Get(ctx, key, ds); err != nil {
		return nil, err
	}

	return ds, nil
}

func (r *DedicatedServerReconciler) getSteamapp(ctx context.Context, ds *v1alpha1.DedicatedServer) (*v1alpha1.Steamapp, error) {
	sa := &v1alpha1.Steamapp{}

	if err := r.Get(ctx, client.ObjectKey{Namespace: ds.Namespace, Name: ds.Spec.Steamapp.Name}, sa); err != nil {
		return nil, err
	}

	return sa, nil
}

func needsHostPort(sa *v1alpha1.Steamapp) bool {
	return xslices.Some(sa.Spec.Ports, func(port v1alpha1.SteamappPort, _ int) bool {
		return len(port.Protocols) > 1
	})
}

// +kubebuilder:rbac:groups=sindri.frantj.cc,resources=dedicatedservers,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=sindri.frantj.cc,resources=dedicatedservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sindri.frantj.cc,resources=dedicatedservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete

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

	sa, err := r.getSteamapp(ctx, ds)
	if err != nil {
		ds.Status.Phase = v1alpha1.PhaseFailed
		return ctrl.Result{}, r.Status().Update(ctx, ds)
	}

	if sa.Status.Phase != v1alpha1.PhaseReady {
		return ctrl.Result{}, nil
	}

	volumeMounts := []corev1.VolumeMount{}
	volumes := []corev1.Volume{}

	for _, vol := range sa.Spec.Volumes {
		var (
			volumeName = fmt.Sprintf("%s-%s", ds.Name, vol.Name)
			pvcSpec    = corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
			}
			pvc = &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      volumeName,
					Namespace: ds.Namespace,
				},
				Spec: pvcSpec,
			}
		)

		volumes = append(volumes, corev1.Volume{
			Name: volumeName,
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: vol.Path,
		})

		if err := controllerutil.SetControllerReference(ds, pvc, r.Scheme()); err != nil {
			ds.Status.Phase = v1alpha1.PhaseFailed
			return ctrl.Result{}, r.Status().Update(ctx, ds)
		}

		if _, err := controllerutil.CreateOrUpdate(ctx, r, pvc, func() error {
			pvc.Spec = pvcSpec
			return controllerutil.SetControllerReference(ds, pvc, r.Scheme())
		}); err != nil {
			ds.Status.Phase = v1alpha1.PhaseFailed
			return ctrl.Result{}, r.Status().Update(ctx, ds)
		}
	}

	var (
		containerPorts = []corev1.ContainerPort{}
		servicePorts   = []corev1.ServicePort{}
	)

	for _, port := range sa.Spec.Ports {
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
		podSpec = corev1.PodSpec{
			Volumes:     volumes,
			HostNetwork: needsHostPort(sa),
			Containers: []corev1.Container{
				{
					Name:         ds.Name,
					Image:        fmt.Sprintf("%s/%d:%s", r.BoilerHost, sa.Spec.AppID, sa.Spec.Branch),
					Ports:        containerPorts,
					VolumeMounts: volumeMounts,
					Resources: corev1.ResourceRequirements{
						Limits:   sa.Spec.Resources,
						Requests: sa.Spec.Resources,
					},
				},
			},
		}
		podLabels = map[string]string{
			"dedicatedserver.sindri.frantj.cc/name": ds.Name,
		}
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ds.Name,
				Namespace: ds.Namespace,
				Labels:    podLabels,
			},
			Spec: podSpec,
		}
	)

	if err := controllerutil.SetControllerReference(ds, pod, r.Scheme()); err != nil {
		ds.Status.Phase = v1alpha1.PhaseFailed
		return ctrl.Result{}, r.Status().Update(ctx, ds)
	}

	if _, err = controllerutil.CreateOrUpdate(ctx, r, pod, func() error {
		pod.Spec = podSpec
		pod.Labels = podLabels
		return controllerutil.SetControllerReference(ds, pod, r.Scheme())
	}); err != nil {
		ds.Status.Phase = v1alpha1.PhaseFailed
		return ctrl.Result{}, r.Status().Update(ctx, ds)
	}

	if !needsHostPort(sa) {
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
			ds.Status.Phase = v1alpha1.PhaseFailed
			return ctrl.Result{}, r.Status().Update(ctx, ds)
		}

		if _, err = controllerutil.CreateOrUpdate(ctx, r, svc, func() error {
			svc.Spec = svcSpec
			return controllerutil.SetControllerReference(ds, svc, r.Scheme())
		}); err != nil {
			ds.Status.Phase = v1alpha1.PhaseFailed
			return ctrl.Result{}, r.Status().Update(ctx, ds)
		}

		for _, ing := range svc.Status.LoadBalancer.Ingress {
			ds.Status.IP = cmp.Or(ing.Hostname, ing.IP)

			if ds.Status.IP != "" {
				break
			}
		}
	} else {
		ds.Status.IP = pod.Status.HostIP
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
		For(&v1alpha1.DedicatedServer{}).
		Complete(r); err != nil {
		return err
	}

	return nil
}
