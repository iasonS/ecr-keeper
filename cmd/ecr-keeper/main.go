// ecr-keeper marks the ECR images that run in a Kubernetes cluster with an extra tag, so a
// lifecycle rule on that tag keeps them.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/iasonS/ecr-keeper/internal/keeper"
	"github.com/iasonS/ecr-keeper/internal/pods"
	"github.com/iasonS/ecr-keeper/internal/registry"
)

func main() {
	if err := run(); err != nil {
		slog.Error("stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cluster := flag.String("cluster", "", "name of this cluster, part of the marker tag (required)")
	prefix := flag.String("prefix", "in-use-", "start of the marker tag")
	mode := flag.String("mode", "report", "report only logs what would change; apply writes tags")
	interval := flag.Duration("interval", 5*time.Minute, "time between rounds")
	grace := flag.Duration("grace", 14*24*time.Hour, "how long a marker stays after its image stops running")
	flag.Parse()

	if *cluster == "" {
		return errors.New("--cluster is required")
	}
	if *mode != "report" && *mode != "apply" {
		return errors.New("--mode must be report or apply")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// In a pod this is the service account; outside it is the current kubeconfig context.
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, nil).ClientConfig()
	if err != nil {
		return err
	}
	kube, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}
	awsConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}

	k := &keeper.Keeper{
		Pods:     &pods.Lister{Client: kube},
		Registry: &registry.ECR{Config: awsConfig},
		Marker:   *prefix + *cluster + "-",
		Grace:    *grace,
		Apply:    *mode == "apply",
		Now:      time.Now,
	}

	slog.Info("started", "marker", k.Marker, "mode", *mode, "interval", interval.String(), "grace", grace.String())
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for {
		if err := k.RunOnce(ctx); err != nil {
			slog.Error("round failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
