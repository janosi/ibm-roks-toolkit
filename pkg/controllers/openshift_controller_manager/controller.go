package openshift_controller_manager

import (
	"context"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	configclient "github.com/openshift/client-go/config/clientset/versioned"
	configinformers "github.com/openshift/client-go/config/informers/externalversions"
	"github.com/openshift/cluster-openshift-controller-manager-operator/pkg/operator/configobservation"
	"github.com/openshift/cluster-openshift-controller-manager-operator/pkg/operator/configobservation/builds"
	"github.com/openshift/cluster-openshift-controller-manager-operator/pkg/operator/configobservation/deployimages"
	"github.com/openshift/cluster-openshift-controller-manager-operator/pkg/operator/configobservation/images"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/configobserver"
	"github.com/openshift/library-go/pkg/operator/events"

	"github.com/openshift/ibm-roks-toolkit/pkg/cmd/cpoperator"
	"github.com/openshift/ibm-roks-toolkit/pkg/controllers"
)

func Setup(cfg *cpoperator.ControlPlaneOperatorConfig) error {
	targetCfg := cfg.TargetConfig()
	kubeInformers := kubeinformers.NewSharedInformerFactoryWithOptions(cfg.TargetKubeClient(), controllers.DefaultResync, kubeinformers.WithNamespace("openshift-controller-manager-operator"))
	configClient, err := configclient.NewForConfig(targetCfg)
	if err != nil {
		return err
	}
	configInformers := configinformers.NewSharedInformerFactory(configClient, controllers.DefaultResync)
	operatorClient := newCMOperatorClient(
		cfg.KubeClient(),
		cfg.Namespace(),
		cfg.Logger().WithName("OpenShiftControllerManagerClient"),
	)

	recorder := events.NewLoggingEventRecorder("openshift-controller-manager-observers")
	c := configobserver.NewConfigObserver(
		operatorClient,
		recorder,
		configobservation.Listers{
			ImageConfigLister: configInformers.Config().V1().Images().Lister(),
			BuildConfigLister: configInformers.Config().V1().Builds().Lister(),
			NetworkLister:     configInformers.Config().V1().Networks().Lister(),
			ConfigMapLister:   kubeInformers.Core().V1().ConfigMaps().Lister(),
			PreRunCachesSynced: []cache.InformerSynced{
				configInformers.Config().V1().Images().Informer().HasSynced,
				configInformers.Config().V1().Builds().Informer().HasSynced,
				configInformers.Config().V1().Networks().Informer().HasSynced,
				kubeInformers.Core().V1().ConfigMaps().Informer().HasSynced,
			},
		},
		[]factory.Informer{},
		images.ObserveInternalRegistryHostname,
		builds.ObserveBuildControllerConfig,
		deployimages.ObserveControllerManagerImagesConfig,
	)
	cfg.Manager().Add(manager.RunnableFunc(func(ctx context.Context) error {
		configInformers.Start(ctx.Done())
		return nil
	}))
	cfg.Manager().Add(manager.RunnableFunc(func(ctx context.Context) error {
		kubeInformers.Start(ctx.Done())
		return nil
	}))
	cfg.Manager().Add(manager.RunnableFunc(func(ctx context.Context) error {
		c.Run(ctx, 1)
		return nil
	}))
	return nil
}
