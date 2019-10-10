package tidb

import (
	"os"

	"github.com/aylei/test-apiserver/pkg/storage"
	"github.com/kubernetes-incubator/apiserver-builder-alpha/pkg/builders"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// NewTidbConfigREST create a customized rest storage for TidbConfg resource
func NewTidbConfigREST(getter generic.RESTOptionsGetter) rest.Storage {
	groupResource := schema.GroupResource{
		Group: "tidb",
		Resource: "tidbconfigs",
	}
	restConfig, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		panic(err)
	}
	strategy := &TidbConfigStrategy{DefaultStorageStrategy: builders.StorageStrategySingleton}

	options, err := getter.GetRESTOptions(groupResource)
	if err != nil {
		panic(err)
	}
	// Replace the default storage with the ConfigMap backed storage
	store := &genericregistry.Store{
		NewFunc:                  func() runtime.Object { return &TidbConfig{} },
		NewListFunc:              func() runtime.Object { return &TidbConfigList{} },
		DefaultQualifiedResource: groupResource,

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
		DeleteStrategy: strategy,

		Storage: genericregistry.DryRunnableStorage{
			Storage: storage.New(clientset, options.StorageConfig.Codec, "default"),
			Codec: options.StorageConfig.Codec,
		},
	}

	storeOptions := &generic.StoreOptions{
		RESTOptions: options,
	}
	if err := store.CompleteWithOptions(storeOptions); err != nil {
		panic(err)
	}
	return &TidbConfigREST{store}
}

// +k8s:deepcopy-gen=false
type TidbConfigREST struct {
	*genericregistry.Store
}
