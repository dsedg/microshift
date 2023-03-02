package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/daemon"
	"github.com/openshift/microshift/pkg/config"
	"github.com/openshift/microshift/pkg/controllers"
	"github.com/openshift/microshift/pkg/kustomize"
	"github.com/openshift/microshift/pkg/loadbalancerservice"
	"github.com/openshift/microshift/pkg/mdns"
	"github.com/openshift/microshift/pkg/node"
	"github.com/openshift/microshift/pkg/servicemanager"
	"github.com/openshift/microshift/pkg/sysconfwatch"
	"github.com/openshift/microshift/pkg/util"
	"github.com/openshift/microshift/pkg/util/cryptomaterial/certchains"
	"github.com/spf13/cobra"

	"k8s.io/klog/v2"
)

const (
	gracefulShutdownTimeout = 60
)

func NewRunMicroshiftCommand() *cobra.Command {
	cfg := config.NewMicroshiftConfig()

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run MicroShift",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunMicroshift(cfg)
		},
	}

	return cmd
}

func RunMicroshift(cfg *config.MicroshiftConfig) error {
	if err := cfg.ReadAndValidate(config.GetConfigFile()); err != nil {
		klog.Fatalf("Error in reading or validating configuration: %v", err)
	}

	// fail early if we don't have enough privileges
	if os.Geteuid() > 0 {
		klog.Fatalf("MicroShift must be run privileged")
	}

	// TO-DO: When multi-node is ready, we need to add the controller host-name/mDNS hostname
	//        or VIP to this list on start
	//        see https://github.com/openshift/microshift/pull/471

	if err := util.AddToNoProxyEnv(
		cfg.NodeIP,
		cfg.NodeName,
		cfg.Cluster.ClusterCIDR,
		cfg.Cluster.ServiceCIDR,
		".svc",
		".cluster.local",
		"."+cfg.BaseDomain); err != nil {
		klog.Fatal(err)
	}

	os.MkdirAll(microshiftDataDir, 0700)

	// TODO: change to only initialize what is strictly necessary for the selected role(s)
	certChains, err := initCerts(cfg)
	if err != nil {
		klog.Fatalf("failed to retrieve the necessary certificates: %v", err)
	}

	// create kubeconfig for kube-scheduler, kubelet,controller-manager
	if err := initKubeconfigs(cfg, certChains); err != nil {
		klog.Fatalf("failed to create the necessary kubeconfigs for internal components: %v", err)
	}

	m := servicemanager.NewServiceManager()
	util.Must(m.AddService(node.NewNetworkConfiguration(cfg)))
	util.Must(m.AddService(controllers.NewEtcd(cfg)))
	util.Must(m.AddService(sysconfwatch.NewSysConfWatchController(cfg)))
	util.Must(m.AddService(controllers.NewKubeAPIServer(cfg)))
	util.Must(m.AddService(controllers.NewKubeScheduler(cfg)))
	util.Must(m.AddService(controllers.NewKubeControllerManager(cfg)))
	util.Must(m.AddService(controllers.NewOpenShiftCRDManager(cfg)))
	util.Must(m.AddService(controllers.NewRouteControllerManager(cfg)))
	util.Must(m.AddService(controllers.NewClusterPolicyController(cfg)))
	util.Must(m.AddService(controllers.NewOpenShiftDefaultSCCManager(cfg)))
	util.Must(m.AddService(mdns.NewMicroShiftmDNSController(cfg)))
	util.Must(m.AddService(controllers.NewInfrastructureServices(cfg)))
	util.Must(m.AddService((controllers.NewVersionManager((cfg)))))
	util.Must(m.AddService(kustomize.NewKustomizer(cfg)))
	util.Must(m.AddService(node.NewKubeletServer(cfg)))
	util.Must(m.AddService(loadbalancerservice.NewLoadbalancerServiceController(cfg)))

	// Storing and clearing the env, so other components don't send the READY=1 until MicroShift is fully ready
	notifySocket := os.Getenv("NOTIFY_SOCKET")
	os.Unsetenv("NOTIFY_SOCKET")

	klog.Infof("Starting MicroShift")

	_, rotationDate, err := certchains.WhenToRotateAtEarliest(certChains)
	if err != nil {
		klog.Fatalf("failed to determine when to rotate certificates: %v", err)
	}

	// TODO: figure out a way to tell the user why the service restarted
	ctx, cancel := context.WithDeadline(context.Background(), rotationDate)
	ready, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		klog.Infof("Started %s", m.Name())
		if err := m.Run(ctx, ready, stopped); err != nil {
			klog.Errorf("Stopped %s: %v", m.Name(), err)
		} else {
			klog.Infof("%s completed", m.Name())

		}
	}()

	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, os.Interrupt, syscall.SIGTERM)

	select {
	case <-ready:
		klog.Infof("MicroShift is ready")
		os.Setenv("NOTIFY_SOCKET", notifySocket)
		if supported, err := daemon.SdNotify(false, daemon.SdNotifyReady); err != nil {
			klog.Warningf("error sending sd_notify readiness message: %v", err)
		} else if supported {
			klog.Info("sent sd_notify readiness message")
		} else {
			klog.Info("service does not support sd_notify readiness messages")
		}

		<-sigTerm
	case <-sigTerm:
	}
	klog.Infof("Interrupt received. Stopping services")
	cancel()

	select {
	case <-stopped:
	case <-sigTerm:
		klog.Infof("Another interrupt received. Force terminating services")
	case <-time.After(time.Duration(gracefulShutdownTimeout) * time.Second):
		klog.Infof("Timed out waiting for services to stop")
	}
	klog.Infof("MicroShift stopped")
	return nil
}
