func getKubernetesClient() (kubernetes.Interface, myresourceclientset.Interface) {
	// construct the path to resolve to `~/.kube/config`
	kubeConfigPath := os.Getenv("HOME") + "/.kube/config"

	// create the config from the path
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		log.Fatalf("getClusterConfig: %v", err)
	}

	// generate the client based off of the config
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("getClusterConfig: %v", err)
	}

	myresourceClient, err := myresourceclientset.NewForConfig(config)
	if err != nil {
		log.Fatalf("getClusterConfig: %v", err)
	}

	log.Info("Successfully constructed k8s client")
	return client, myresourceClient
}

func main() {
	// get the Kubernetes client for connectivity
	client, myresourceClient := getKubernetesClient()

	// retrieve our custom resource informer which was generated from
	// the code generator and pass it the custom resource client, specifying
	// we should be looking through all namespaces for listing and watching
	informer := myresourceinformer_v1.NewMyResourceInformer(
		myresourceClient,
		meta_v1.NamespaceAll,
		0,
		cache.Indexers{},
	)

	// ... remainder of main.main unchanged and omitted for brevity
}
