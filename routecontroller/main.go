package main

import (
	"flag"
	"fmt"
	"time"

	"k8s.io/klog/v2"

	secret "github.com/chiragkyal/watch-based-informer/monitorv4"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	// routev1 "github.com/openshift/api/route/v1"
	// routev1client "github.com/openshift/client-go/route/clientset/versioned"
)

var namespace = "sandbox"

// Controller demonstrates how to implement a controller with client-go.
type Controller struct {
	indexer  cache.Indexer
	queue    workqueue.RateLimitingInterface
	informer cache.Controller
}

// NewController creates a new Controller.
func NewController(queue workqueue.RateLimitingInterface, indexer cache.Indexer, informer cache.Controller, clientset *kubernetes.Clientset /*handlerFuncs cache.ResourceEventHandlerFuncs*/) *Controller {

	return &Controller{
		informer: informer,
		indexer:  indexer,
		queue:    queue,
	}
}

func (c *Controller) processNextItem() bool {
	// Wait until there is a new item in the working queue
	item, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(item)

	if _, ok := item.(Handler); ok {
		klog.Info("successfully got item %v", item)
		// return true
	} else {
		klog.Info("did not got item %v", item)
	}

	// Invoke the method containing the business logic
	err := c.syncToStdout(item.(Handler))
	klog.Error(err)

	// Handle the error if something went wrong during the execution of the business logic
	// 	TODO: invoke handle err
	// c.handleErr(err, key)
	return true
}

// syncToStdout is the business logic of the controller. In this controller it simply prints
// information about the pod to stdout. In case an error happened, it has to simply return the error.
// The retry logic should not be part of the business logic.
func (c *Controller) syncToStdout(handler Handler) error {
	obj, exists, err := c.indexer.GetByKey(handler.GetResourceKey())
	if err != nil {
		klog.Errorf("Fetching object with key %s from store failed with %v", handler.GetResourceKey(), err)
		return err
	}

	if !exists {
		// Below we will warm up our cache with a Pod, so that we will see a delete for one pod
		klog.Info("Route %s does not exist anymore\n", handler.GetResourceKey())
	} else {
		// Note that you also have to check the uid if you have a local controlled resource, which
		// is dependent on the actual instance, to detect that a Pod was recreated with the same name
		if _, ok := obj.(*v1.Pod); !ok {
			klog.Error("unable to type cast to pod")
			return err
		}
		pod := obj.(*v1.Pod)

		klog.Info("Sync/Add/Update for Route %s\n", pod.GetName())

		// invoke actual logic
		if err := handler.HandleRoute( /*handler.GetEventType()*/ pod); err != nil {
			return err
		}
	}
	return nil
}

// handleErr checks if an error happened and makes sure we will retry later.
func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		c.queue.Forget(key)
		return
	}

	// This controller retries 5 times if something goes wrong. After that, it stops trying.
	if c.queue.NumRequeues(key) < 5 {
		klog.Infof("Error syncing pod %v: %v", key, err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	utilruntime.HandleError(err)
	klog.Infof("Dropping pod %q out of the queue: %v", key, err)
}

// Run begins watching and syncing.
func (c *Controller) Run(workers int, stopCh chan struct{}) {
	defer utilruntime.HandleCrash()

	// Let the workers stop when we are done
	defer c.queue.ShutDown()
	klog.Info("Starting Route controller")

	go c.informer.Run(stopCh)

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	klog.Info("Stopping Pod controller")
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

func getReferenceSecret(route *v1.Pod /*routev1.Route*/) string {
	// route.Spec.TLS.ExternalCertificate.Name
	secretName := route.Spec.Containers[0].Env[0].ValueFrom.SecretKeyRef.Name
	klog.V(2).Info("Referenced secretName: ", secretName)
	return secretName
}

func main() {
	var kubeconfig string
	var master string

	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
	flag.Parse()

	// creates the connection
	config, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		klog.Fatal(err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	// routeListWatcher := cache.NewListWatchFromClient(routev1client.NewForConfigOrDie(config).RouteV1().RESTClient(), "routes", namespace, fields.Everything())
	routeListWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "pods", namespace, fields.Everything())

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	secretManager := secret.NewManager(clientset, queue)

	// Route handler
	routeh := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			var key string
			if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
				panic(err)
			}
			route := obj.(*v1.Pod)
			klog.Info("Add Event ", "route.Name ", route.Name, " key ", key)

			queue.Add(NewRouteEvent(watch.Added, key, secretManager))
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			var oldKey, newKey string
			var err error
			if newKey, err = cache.MetaNamespaceKeyFunc(new); err != nil {
				panic(err)
			}
			if oldKey, err = cache.MetaNamespaceKeyFunc(old); err != nil {
				panic(err)
			}
			oldRoute := old.(*v1.Pod)
			newRoute := new.(*v1.Pod)

			if getReferenceSecret(oldRoute) != getReferenceSecret(newRoute) {
				klog.Info("Roue Update event ", "old ", oldRoute.ResourceVersion, " new ", newRoute.ResourceVersion, " newkey ", newKey, " oldKey ", oldKey)
				// remove old watch
				queue.Add(NewRouteEvent(watch.Deleted, oldKey, secretManager))
				// create new watch
				queue.Add(NewRouteEvent(watch.Added, newKey, secretManager))
			}
		},
		DeleteFunc: func(obj interface{}) {
			var key string
			if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
				panic(err)
			}
			route := obj.(*v1.Pod)
			klog.Info("Delete event ", " obj ", route.ResourceVersion, " key ", key)

			// when route is deleted, remove associated secret watcher
			// TODO: Here queue will ingore this call and will drop this route because it won't be able to this route from cache after it got deleted
			// and hence secret manager would not be called.
			// TODO: should be directly pass the object?
			queue.Add(NewRouteEvent(watch.Deleted, key, secretManager))

		},
	}

	// Route Controller
	indexer, informer := cache.NewIndexerInformer(routeListWatcher, &v1.Pod{} /*&routev1.Route{}*/, 0, routeh, cache.Indexers{})

	controller := NewController(queue, indexer, informer, clientset)

	// Now let's start the controller
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)

	// Wait forever
	select {}
}
