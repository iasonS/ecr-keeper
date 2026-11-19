// Package pods lists the ECR images that the pods of a cluster run.
package pods

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/iasonS/ecr-keeper/internal/images"
)

// Lister reads pods from every namespace.
type Lister struct {
	Client kubernetes.Interface
}

// Running returns the ECR images of all pods that have not finished.
func (l *Lister) Running(ctx context.Context) ([]images.Ref, error) {
	pods, err := l.Client.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var refs []images.Ref
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		statuses := append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...)
		for _, status := range statuses {
			if ref, found := images.Parse(status.Image, status.ImageID); found {
				refs = append(refs, ref)
			}
		}
	}
	return refs, nil
}
