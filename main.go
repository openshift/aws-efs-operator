/*
Copyright 2022.

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

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	
	"k8s.io/client-go/rest"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	// "openshift/aws-efs-operator/api"
	awsefsControllers "openshift/aws-efs-operator/controllers"
	
	statics "openshift/aws-efs-operator/controllers/statics"
	"openshift/aws-efs-operator/version"
	awsefsAPI "openshift/aws-efs-operator/api"
	awsefsConfig "openshift/aws-efs-operator/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	monclientv1 "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	"openshift/aws-efs-operator/pkg/k8sutil"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	// "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	opmetrics "github.com/openshift/operator-custom-metrics/pkg/metrics"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	// "sigs.k8s.io/controller-runtime/pkg/healthz"

	awsefsv1alpha1 "openshift/aws-efs-operator/api/v1alpha1"

	//+kubebuilder:scaffold:imports
)

var (
	// metricsHost               = "0.0.0.0"
	metricsPort         int32 = 8383
	// operatorMetricsPort int32 = 8686
	customMetricsPath       = "/metrics"
	scheme                    = apiruntime.NewScheme()
	log = ctrl.Log.WithName("cmd")
)


func printVersion() {
	log.Info(fmt.Sprintf("Operator Version: %s", version.Version))
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("Version of operator-sdk: %v", version.SDKVersion))
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(awsefsv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var probeAddr string
	// var enableLeaderElection bool
	enableLeaderElection := true

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":"+fmt.Sprintf("%d", metricsPort), "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	// logf.SetLogger(zap.Logger())

	printVersion()

	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		log.Error(err, "Failed to get watch namespace")
		os.Exit(1)
	}
	if namespace != "" {
		log.Info(fmt.Sprintf(
			"This operator watches all namespaces. Ignoring configured watch namespace '%s'.",
			namespace))
		namespace = ""
	}
	// This set the sync period to 5m
	syncPeriod := awsefsConfig.SyncPeriodDefault
	

	/*new Manager*/
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Namespace:              namespace,
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "312e6264.managed.openshift.io",
		SyncPeriod:             &syncPeriod,
		NewClient: func(_ cache.Cache, config *rest.Config, options client.Options, uncachedObjects ...client.Object) (client.Client, error) {
			return client.New(config, options)
		},
	})

	log.Info("Registering Components.")
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}


	// Setup Scheme for all resources
	if err := awsefsAPI.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Add OpenShift security apis to scheme. TODO
	// if err := awsefsAPI.Install(mgr.GetScheme()); err != nil {
	// 	log.Error(err, "")
	// 	os.Exit(1)
	// }

	// Need this for the CustomResourceDefinition Kind
	if err := apiextensions.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Setup all Controllers. TODO
	// if err := awsefsControllers.AddToManager(mgr); err != nil {
	// 	log.Error(err, "")
	// 	os.Exit(1)
	// }

	// Get a config to talk to the apiserver
	ctx := context.TODO()
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Add the Custom Metrics Service
	metricsClient, err := client.New(cfg, client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		log.Error(err, "unable to create k8s client for upgrade metrics")
		os.Exit(1)
	}
	
	// Add the Metrics Service and ServiceMonitor
	if err := addMetrics(ctx, metricsClient, cfg); err != nil {
		log.Error(err, "Metrics service is not added.")
		os.Exit(1)
	}

	ns, err := awsefsControllers.GetOperatorNamespace()
	if err != nil {
		os.Exit(1)
	}

	customMetrics := opmetrics.NewBuilder(ns, "managed-upgrade-operator-custom-metrics").
		WithPath(customMetricsPath).
		WithServiceMonitor().
		WithServiceLabel(map[string]string{"name": awsefsConfig.OperatorName}).
		GetConfig()

	if err = opmetrics.ConfigureMetrics(context.TODO(), *customMetrics); err != nil {
		log.Error(err, "Failed to configure custom metrics")
		os.Exit(1)
	}
	
	// Create k8s client to perform startup tasks.
	startupClient, err := client.New(cfg, client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		log.Error(err, "Unable to create operator startup client")
		os.Exit(1)
	}

	// Ensure static resources are created.
	if err := statics.EnsureStatics(log, startupClient); err != nil {
		log.Error(err, "Couldn't bootstrap static resources")
		os.Exit(1)
	}


	log.Info("Starting the Cmd.")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "Manager exited non-zero")
		os.Exit(1)
	}
}

// addMetrics will create the Services and Service Monitors to allow the operator export the metrics by using
// the Prometheus operator

func addMetrics(ctx context.Context, cl client.Client, cfg *rest.Config) error {
	// Get the namespace the operator is currently deployed in.
	operatorNs, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		if errors.Is(err, k8sutil.ErrRunLocal) || errors.Is(err, k8sutil.ErrNoNamespace) {
			log.Info("Skipping metrics service creation; not running in a cluster.")
			return nil
		}
	}

	// Get the operator name
	operatorName, _ := k8sutil.GetOperatorName()

	service, err := opmetrics.GenerateService(metricsPort, "http-metrics", operatorName+"-metrics", operatorNs, map[string]string{"name": operatorName})
	if err != nil {
		log.Info("Could not create metrics Service", "error", err.Error())
		return err
	}

	log.Info(fmt.Sprintf("Attempting to create service %s", service.Name))
	err = cl.Create(ctx, service)
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			log.Error(err, "Could not create metrics service")
			return err
		} else {
			log.Info("Metrics service already exists, will not create")
		}
	}

	services := []*corev1.Service{service}
	mclient := monclientv1.NewForConfigOrDie(cfg)
	copts := metav1.CreateOptions{}

	for _, s := range services {
		if s == nil {
			continue
		}

		sm := opmetrics.GenerateServiceMonitor(s)

		// ErrSMMetricsExists is used to detect if the -metrics ServiceMonitor already exists
		var ErrSMMetricsExists = fmt.Sprintf("servicemonitors.monitoring.coreos.com \"%s-metrics\" already exists", awsefsConfig.OperatorName)

		log.Info(fmt.Sprintf("Attempting to create service monitor %s", sm.Name))
		// TODO: Get SM and compare to see if an UPDATE is required
		_, err := mclient.ServiceMonitors(operatorNs).Create(ctx, sm, copts)
		if err != nil {
			if err.Error() != ErrSMMetricsExists {
				return err
			}
			log.Info("ServiceMonitor already exists")
		}
		log.Info(fmt.Sprintf("Successfully configured service monitor %s", sm.Name))
	}
	return nil
}

