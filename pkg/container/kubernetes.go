package container

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/transferia/transferia/library/go/core/xerrors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type K8sWrapper struct {
	client kubernetes.Interface
}

func NewK8sWrapper() (*K8sWrapper, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, xerrors.Errorf("failed to load in-cluster config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, xerrors.Errorf("failed to create k8s client: %w", err)
	}
	return &K8sWrapper{client: clientset}, nil
}

func (w *K8sWrapper) Pull(_ context.Context, _ string, _ types.ImagePullOptions) error {
	// No need to pull images in k8s
	return nil
}

func (w *K8sWrapper) getCurrentNamespace() (string, error) {
	b, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// Helper method to create a pod
func (w *K8sWrapper) createPod(ctx context.Context, opts K8sOpts) (*corev1.Pod, error) {
	if opts.Namespace == "" {
		ns, err := w.getCurrentNamespace()
		if err != nil {
			ns = "default"
		}
		opts.Namespace = ns
	}

	if opts.PodName == "" {
		opts.PodName = "transferia-runner"
	}

	if opts.ContainerName == "" {
		opts.ContainerName = "runner"
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", opts.PodName),
			Namespace:    opts.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:         opts.ContainerName,
					Image:        opts.Image,
					Command:      opts.Command,
					Args:         opts.Args,
					Env:          opts.Env,
					VolumeMounts: opts.VolumeMounts,
				},
			},
			Volumes:       opts.Volumes,
			RestartPolicy: opts.RestartPolicy,
		},
	}

	return w.client.CoreV1().Pods(opts.Namespace).Create(ctx, pod, metav1.CreateOptions{})
}

func (w *K8sWrapper) Run(ctx context.Context, opts ContainerOpts) (stdout io.ReadCloser, stderr io.ReadCloser, err error) {
	// Convert options to K8s options
	k8sOpts := opts.ToK8sOpts()

	// Create the pod
	pod, err := w.createPod(ctx, k8sOpts)
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to create pod: %w", err)
	}

	// Channel to signal when log streaming is done
	logStreamingDone := make(chan struct{})
	podDeleted := make(chan struct{})

	// Launch a goroutine to monitor pod completion and cleanup
	go func() {
		defer close(podDeleted)

		watchPod := func() {
			timeout := time.After(k8sOpts.Timeout)
			tick := time.NewTicker(2 * time.Second)
			defer tick.Stop()

			for {
				select {
				case <-ctx.Done():
					// Context cancelled, wait for log streaming to finish before cleanup
					select {
					case <-logStreamingDone:
						// Log streaming is done
					case <-time.After(10 * time.Second):
						// Give log streaming 10 seconds to finish
					}
					_ = w.client.CoreV1().Pods(pod.GetNamespace()).Delete(
						context.Background(), pod.GetName(), metav1.DeleteOptions{})
					return

				case <-timeout:
					// Timeout reached, wait for log streaming to finish before cleanup
					select {
					case <-logStreamingDone:
						// Log streaming is done
					case <-time.After(10 * time.Second):
						// Give log streaming 10 seconds to finish
					}
					_ = w.client.CoreV1().Pods(pod.GetNamespace()).Delete(
						context.Background(), pod.GetName(), metav1.DeleteOptions{})
					return

				case <-tick.C:
					p, err := w.client.CoreV1().Pods(pod.GetNamespace()).Get(
						ctx, pod.GetName(), metav1.GetOptions{})
					if err != nil {
						if apierrors.IsNotFound(err) {
							return // Pod already deleted, nothing to do
						}
						continue // Other error, keep monitoring
					}

					phase := p.Status.Phase
					if phase == corev1.PodSucceeded || phase == corev1.PodFailed {
						// Pod completed
						// Wait for log streaming to finish
						select {
						case <-logStreamingDone:
							// Log streaming is done, safe to delete
						case <-time.After(30 * time.Second):
							// Give logger 30 seconds to finish reading logs
						}
						// Then clean up
						_ = w.client.CoreV1().Pods(pod.GetNamespace()).Delete(
							context.Background(), pod.GetName(), metav1.DeleteOptions{})
						return
					}
				}
			}
		}
		watchPod()
	}()

	// Wait for pod to reach Running state before trying to stream logs
	if err := w.waitForPodReady(ctx, pod.GetNamespace(), pod.GetName(), 30*time.Second); err != nil {
		// If pod can't get to running state, try to get logs anyway
		// but log a warning
		fmt.Printf("Warning: pod %s may not be ready: %v\n", pod.GetName(), err)
	}

	// Set up log streaming options
	logOpts := &corev1.PodLogOptions{
		Container: k8sOpts.ContainerName,
		Follow:    true,
	}

	// Get logs stream
	req := w.client.CoreV1().Pods(pod.GetNamespace()).GetLogs(pod.GetName(), logOpts)
	stream, err := req.Stream(ctx)
	if err != nil {
		// If we can't get logs, close the logStreamingDone channel to allow pod cleanup
		close(logStreamingDone)

		// Wait for pod to be deleted
		select {
		case <-podDeleted:
			// Pod was deleted
		case <-time.After(5 * time.Second):
			// Don't wait too long
		}

		return nil, nil, xerrors.Errorf("failed to stream pod logs: %w", err)
	}

	// Wrap the stream to signal when it's closed
	wrappedStream := &streamWrapper{
		ReadCloser: stream,
		onClose: func() {
			close(logStreamingDone)
		},
	}

	// Return the wrapped stream
	return wrappedStream, nil, nil
}

// Helper to wait for pod to be ready
func (w *K8sWrapper) waitForPodReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return xerrors.New("timeout waiting for pod to be ready")

		case <-time.After(1 * time.Second):
			pod, err := w.client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					continue // Pod might not be created yet
				}
				return xerrors.Errorf("error getting pod: %w", err)
			}

			// If pod is running or succeeded/failed, it's ready for logs
			if pod.Status.Phase == corev1.PodRunning ||
				pod.Status.Phase == corev1.PodSucceeded ||
				pod.Status.Phase == corev1.PodFailed {
				return nil
			}

			// If pod is in error state, return an error
			if pod.Status.Phase == corev1.PodUnknown {
				return xerrors.New("pod is in Unknown phase")
			}
		}
	}
}

// RunAndWait reads all logs from a pod until completion
func (w *K8sWrapper) RunAndWait(ctx context.Context, opts ContainerOpts) (*bytes.Buffer, *bytes.Buffer, error) {
	stdoutReader, _, err := w.Run(ctx, opts)
	if err != nil {
		return nil, nil, err
	}
	defer stdoutReader.Close()

	stdoutBuf := new(bytes.Buffer)

	_, err = io.Copy(stdoutBuf, stdoutReader)
	if err != nil && err != io.EOF {
		return stdoutBuf, nil, xerrors.Errorf("error copying pod logs: %w", err)
	}

	return stdoutBuf, nil, nil
}
