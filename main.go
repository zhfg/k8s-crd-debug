package main

import (
	"encoding/json"
	"flag"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	// "k8s.io/kubernetes/staging/src/k8s.io/metrics/pkg/client/clientset_generated/clientset"
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"github.com/zhfg/k8s-crd-debug/pkg/signals"

	clientset "github.com/zhfg/k8s-crd-debug/pkg/client/clientset/versioned"
	informers "github.com/zhfg/k8s-crd-debug/pkg/client/informers/externalversions"
)

var (
	masterURL  string
	kubeconfig string
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	klog.Info(cfg)

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}
	klog.Info(json.Marshal(kubeClient))

	exampleClient, err := clientset.NewForConfig(cfg)

	klog.Info(json.Marshal(exampleClient))
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}
	klog.Info(exampleClient)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	exampleInformerFactory := informers.NewSharedInformerFactory(exampleClient, time.Second*30)
	// klog.Info(kubeInformerFactory)
	// klog.Info(exampleInformerFactory)

	controller := NewController(kubeClient, exampleClient,
		kubeInformerFactory.Apps().V1().Deployments(),
		exampleInformerFactory.Debuger().V1().DebugerTypes())

	kubeInformerFactory.Start(stopCh)
	exampleInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
