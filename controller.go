package main

import (
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
	// SuccessSynced is used as part of the Event 'reason' when a Debugger is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Debugger fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Debugger"
	// MessageResourceSynced is the message used for an Event fired when a Debugger
	// is synced successfully
	MessageResourceSynced = "Debugger synced successfully"
)

// Controller is the controller implementation for Debugger resources
type Controller struct {
	kubeclientset kubernetes.Interface

	debuggerclientset clientset.Interface

	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced
	debuggerLister    listers.DebugerTypeLister
	debuggerSynced    cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
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

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
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
	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a Debugger resource will enqueue that Debugger resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*appsv1.Deployment)
			oldDepl := old.(*appsv1.Deployment)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
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

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Debugger resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
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

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Debugger resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	return nil
	// // Convert the namespace/name string into a distinct namespace and name
	// namespace, name, err := cache.SplitMetaNamespaceKey(key)
	// if err != nil {
	// 	utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
	// 	return nil
	// }

	// // Get the Debugger resource with this namespace/name
	// Debugger, err := c.debuggerLister.DebugerTypes(namespace).Get(name)

	// if err != nil {
	// 	// The Debugger resource may no longer exist, in which case we stop
	// 	// processing.
	// 	if errors.IsNotFound(err) {
	// 		utilruntime.HandleError(fmt.Errorf("Debugger '%s' in work queue no longer exists", key))
	// 		return nil
	// 	}

	// 	return err
	// }

	// deploymentName := Debugger.Spec.DeployName
	// if deploymentName == "" {
	// 	// We choose to absorb the error here as the worker would requeue the
	// 	// resource otherwise. Instead, the next time the resource is updated
	// 	// the resource will be queued again.
	// 	utilruntime.HandleError(fmt.Errorf("%s: deployment name must be specified", key))
	// 	return nil
	// }

	// // Get the deployment with the name specified in Debugger.spec
	// deployment, err := c.deploymentsLister.Deployments(Debugger.Namespace).Get(deploymentName)
	// // If the resource doesn't exist, we'll create it
	// if errors.IsNotFound(err) {
	// 	deployment, err = c.kubeclientset.AppsV1().Deployments(Debugger.Namespace).Create(newDeployment(Debugger), metav1.CreateOptions{})
	// }

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
	// 	deployment, err = c.kubeclientset.AppsV1().Deployments(Debugger.Namespace).Update(context.TODO(), newDeployment(Debugger), metav1.UpdateOptions{})
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
	// return nil
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
	// _, err := c.debuggerclientset.DebugerV1.DebugerTypes(Debugger.Namespace).Update(DebuggerCopy, metav1.UpdateOptions{})
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

// newDeployment creates a new Deployment for a Debugger resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Debugger resource that 'owns' it.
func newDeployment(Debugger *debugv1.DebugerType) *appsv1.Deployment {
	// labels := map[string]string{
	// 	"app":        "nginx",
	// 	"controller": Debugger.Name,
	// }
	// return &appsv1.Deployment{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name:      Debugger.Spec.DeployName,
	// 		Namespace: Debugger.Namespace,
	// 		OwnerReferences: []metav1.OwnerReference{
	// 			*metav1.NewControllerRef(Debugger, debugv1.SchemeGroupVersion.WithKind("Debugger")),
	// 		},
	// 	},
	// 	Spec: appsv1.DeploymentSpec{
	// 		Replicas: Debugger.Spec.Replicas,
	// 		Selector: &metav1.LabelSelector{
	// 			MatchLabels: labels,
	// 		},
	// 		Template: corev1.PodTemplateSpec{
	// 			ObjectMeta: metav1.ObjectMeta{
	// 				Labels: labels,
	// 			},
	// 			Spec: corev1.PodSpec{
	// 				Containers: []corev1.Container{
	// 					{
	// 						Name:  "nginx",
	// 						Image: "nginx:latest",
	// 					},
	// 				},
	// 			},
	// 		},
	// 	},
	// }
	return nil
}