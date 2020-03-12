package main

import (
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	debugv1 "github.com/zhfg/k8s-crd-debug/pkg/apis/debuger/v1"

	clientset "github.com/zhfg/k8s-crd-debug/pkg/client/clientset/versioned"
	debuggerscheme "github.com/zhfg/k8s-crd-debug/pkg/client/clientset/versioned/scheme"
	informers "github.com/zhfg/k8s-crd-debug/pkg/client/informers/externalversions/debuger/v1"
	listers "github.com/zhfg/k8s-crd-debug/pkg/client/listers/debuger/v1"
)

const controllerAgentName = "debugger"

const (
	SuccessSynced = "Synced"

	ErrResourceExists = "ErrResourceExists"

	MessageResourceExists = "Resource %q already exists and is not managed by Debugger"

	MessageResourceSynced = "Debugger synced successfully"

	SidecarImageName         = "theiaide/theia:0.16.1"
	SidecarContainerName     = "sidecar-theia"
	ContainerShareVolumeName = "share-data"
)

// Controller is the controller implementation for Debugger resources
type Controller struct {
	kubeclientset kubernetes.Interface

	debuggerclientset clientset.Interface

	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced
	debuggerLister    listers.DebugerTypeLister
	debuggerSynced    cache.InformerSynced

	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	debuggerclientset clientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	debuggerInformer informers.DebugerTypeInformer) *Controller {

	utilruntime.Must(debuggerscheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:     kubeclientset,
		debuggerclientset: debuggerclientset,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		debuggerLister:    debuggerInformer.Lister(),
		debuggerSynced:    debuggerInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Debuggers"),
		recorder:          recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when Debugger resources change
	debuggerInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueDebugger,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueDebugger(new)
		},
	})

	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*appsv1.Deployment)
			oldDepl := old.(*appsv1.Deployment)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {

				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// Run
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Debugger controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.debuggerSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process Debugger resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {

		defer c.workqueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {

			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}

		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) syncHandler(key string) error {
	klog.Info(key)
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	klog.Info(fmt.Sprintf("Get a new Debugger, namespaces is %s and name is %s", namespace, name))

	//Get the Debugger resource with this namespace/name
	Debugger, err := c.debuggerLister.DebugerTypes(namespace).Get(name)

	if err != nil {
		// The Debugger resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("Debugger '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	deploymentName := Debugger.Spec.DeploymentName
	if deploymentName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		utilruntime.HandleError(fmt.Errorf("%s: deployment name must be specified", key))
		return nil
	}
	// check if share volume not provided
	shareVolume := Debugger.Spec.ShareVolumeName
	if shareVolume == "" {
		utilruntime.HandleError(fmt.Errorf("%s: share data volume name must be specified", key))
		return nil
	}

	deployment, err := c.deploymentsLister.Deployments(Debugger.Namespace).Get(deploymentName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		utilruntime.HandleError(fmt.Errorf("could not find deployment %s/%s, maybe it is not existing", Debugger.Namespace, deploymentName))
		return nil
		// deployment, err = c.kubeclientset.AppsV1().Deployments(Debugger.Namespace).Create(newDeployment(Debugger), metav1.CreateOptions{})
	}
	klog.Warning(deployment, err)
	// return if do not have shared volume

	if !checkVolumeExisting(deployment, shareVolume) {
		utilruntime.HandleError(fmt.Errorf("deployment %s/%s do not have existed share volume", Debugger.Namespace, deploymentName))
		return nil
	}
	if checkSidecarContainerExisting(deployment) {
		utilruntime.HandleError(fmt.Errorf("it seems the deployment %s/%s have a sidecar named %s already, skipping", deployment.GetNamespace(), deploymentName, SidecarContainerName))
		return nil
	}
	err = c.attachSidecar(deployment)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("attach sidecar to deployment %s failed", key))
		return nil
	}
	err = c.updateDebuggerStatus(Debugger, deployment)
	if err != nil {
		return err
	}

	// // If an error occurs during Get/Create, we'll requeue the item so we can
	// // attempt processing again later. This could have been caused by a
	// // temporary network failure, or any other transient reason.
	// if err != nil {
	// 	return err
	// }

	// // If the Deployment is not controlled by this Debugger resource, we should log
	// // a warning to the event recorder and return error msg.
	// if !metav1.IsControlledBy(deployment, Debugger) {
	// 	msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
	// 	c.recorder.Event(Debugger, corev1.EventTypeWarning, ErrResourceExists, msg)
	// 	return fmt.Errorf(msg)
	// }

	// // If this number of the replicas on the Debugger resource is specified, and the
	// // number does not equal the current desired replicas on the Deployment, we
	// // should update the Deployment resource.
	// if Debugger.Spec.Replicas != nil && *Debugger.Spec.Replicas != *deployment.Spec.Replicas {
	// 	klog.V(4).Infof("Debugger %s replicas: %d, deployment replicas: %d", name, *Debugger.Spec.Replicas, *deployment.Spec.Replicas)
	// deployment, err = c.kubeclientset.AppsV1().Deployments(Debugger.Namespace).Update(context.TODO(), newDeployment(Debugger), metav1.UpdateOptions{})
	// }

	// // If an error occurs during Update, we'll requeue the item so we can
	// // attempt processing again later. This could have been caused by a
	// // temporary network failure, or any other transient reason.
	// if err != nil {
	// 	return err
	// }

	// // Finally, we update the status block of the Debugger resource to reflect the
	// // current state of the world
	// err = c.updateDebuggerStatus(Debugger, deployment)
	// if err != nil {
	// 	return err
	// }

	// c.recorder.Event(Debugger, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func checkVolumeExisting(deployment *appsv1.Deployment, sharedVolumeName string) bool {
	sharedDataVolumes := deployment.Spec.Template.Spec.Volumes
	for _, volume := range sharedDataVolumes {
		if volume.Name == sharedVolumeName {
			return true
		}
	}
	return false
}

func checkSidecarContainerExisting(deployment *appsv1.Deployment) bool {
	containers := deployment.Spec.Template.Spec.Containers
	for _, con := range containers {
		if con.Name == SidecarContainerName {
			return true
		}
	}
	return false
}

func checkContainerHasShareVolume(deployment *appsv1.Deployment) bool {
	containers := deployment.Spec.Template.Spec.Containers
	for _, con := range containers {
		if con.Name != SidecarContainerName {
			volumeMounts := con.VolumeMounts
			for _, volumeMount := range volumeMounts {
				if volumeMount.Name == ContainerShareVolumeName {
					return true
				}
			}
		}
	}
	return false
}

func (c *Controller) attachSidecar(deployment *appsv1.Deployment) error {
	klog.Info("Attaching Starting")
	deploymentNamespace := deployment.GetNamespace()
	// deploymentName := deployment.GetName()
	newDeployment := updateDeployment(deployment)
	deployment, err := c.kubeclientset.AppsV1().Deployments(deploymentNamespace).Update(newDeployment)
	if err != nil {
		klog.Error(fmt.Sprintf("Can not update deployment %s-%s, error is %s", deploymentNamespace, deployment.GetName(), err))
		return err
	}

	// Get shared volume from deployment
	klog.Info("Attached")
	return nil
}
func (c *Controller) updateDebuggerStatus(Debugger *debugv1.DebugerType, deployment *appsv1.Deployment) error {
	// // NEVER modify objects from the store. It's a read-only, local cache.
	// // You can use DeepCopy() to make a deep copy of original object and modify this copy
	// // Or create a copy manually for better performance
	// DebuggerCopy := Debugger.DeepCopy()
	// DebuggerCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas
	// // If the CustomResourceSubresources feature gate is not enabled,
	// // we must use Update instead of UpdateStatus to update the Status block of the Debugger resource.
	// // UpdateStatus will not allow changes to the Spec of the resource,
	// // which is ideal for ensuring nothing other than resource status has been updated.
	// _, err := c.debuggerclientset.DebugerV1.DebugerTypes(Debugger.Message).Update(DebuggerCopy, metav1.UpdateOptions{})
	// return err
	return nil
}

// enqueueDebugger takes a Debugger resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Debugger.
func (c *Controller) enqueueDebugger(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Debugger resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Debugger resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	// var object metav1.Object
	// var ok bool
	// if object, ok = obj.(metav1.Object); !ok {
	// 	tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
	// 	if !ok {
	// 		utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
	// 		return
	// 	}
	// 	object, ok = tombstone.Obj.(metav1.Object)
	// 	if !ok {
	// 		utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
	// 		return
	// 	}
	// 	klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	// }
	// klog.V(4).Infof("Processing object: %s", object.GetName())
	// if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
	// 	// If this object is not owned by a Debugger, we should not do anything more
	// 	// with it.
	// 	if ownerRef.Kind != "Debugger" {
	// 		return
	// 	}

	// 	Debugger, err := c.DebuggersLister.DdbugType(object.GetNamespace()).Get(ownerRef.Name)
	// 	if err != nil {
	// 		klog.V(4).Infof("ignoring orphaned object '%s' of Debugger '%s'", object.GetSelfLink(), ownerRef.Name)
	// 		return
	// 	}

	// 	c.enqueueDebugger(Debugger)
	// 	return
	// }
}

// updateDeployment creates a update Deployment for a Debugger resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Debugger resource that 'owns' it.
func updateDeployment(deployment *appsv1.Deployment) *appsv1.Deployment {
	sharedDataVolumes := deployment.Spec.Template.Spec.Volumes
	klog.Info(sharedDataVolumes)
	// Create a new container
	sidecarContainer := corev1.Container{}
	sidecarContainer.Name = SidecarContainerName
	sidecarContainer.Image = SidecarImageName

	// attach share-data to sidecar container
	shareDataVolumeMount := corev1.VolumeMount{}
	shareDataVolumeMount.Name = ContainerShareVolumeName
	shareDataVolumeMount.MountPath = "/data/"

	sidecarContainer.VolumeMounts = append(sidecarContainer.VolumeMounts, shareDataVolumeMount)
	deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, sidecarContainer)
	klog.Info(deployment)
	return deployment
}
