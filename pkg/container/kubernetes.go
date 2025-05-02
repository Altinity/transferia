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
	"go.ytsaurus.tech/library/go/core/log"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type K8sWrapper struct {
	client kubernetes.Interface
	logger log.Logger
}

func NewK8sWrapper(logger log.Logger) (*K8sWrapper, error) {
	logger.Info("Initializing Kubernetes wrapper")
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Errorf("Failed to load in-cluster config: %v", err)
		return nil, xerrors.Errorf("failed to load in-cluster config: %w", err)
	}
	logger.Info("Successfully loaded in-cluster config")

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Errorf("Failed to create Kubernetes client: %v", err)
		return nil, xerrors.Errorf("failed to create k8s client: %w", err)
	}
	logger.Info("Successfully created Kubernetes client")

	return &K8sWrapper{client: clientset, logger: logger}, nil
}

func (w *K8sWrapper) Pull(_ context.Context, _ string, _ types.ImagePullOptions) error {
	// No need to pull images in k8s
	return nil
}

func (w *K8sWrapper) getCurrentNamespace() (string, error) {
	w.logger.Info("Getting current namespace from service account")
	b, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		w.logger.Warnf("Failed to read namespace from service account: %v", err)
		return "", err
	}
	namespace := string(b)
	w.logger.Infof("Current namespace: %s", namespace)
	return namespace, nil
}

// Helper method to create a pod
func (w *K8sWrapper) createPod(ctx context.Context, opts K8sOpts) (*corev1.Pod, error) {
	w.logger.Info("Creating pod with options")

	if opts.Namespace == "" {
		ns, err := w.getCurrentNamespace()
		if err != nil {
			ns = "default"
			w.logger.Infof("Using default namespace due to error: %v", err)
		}
		opts.Namespace = ns
		w.logger.Infof("Using namespace: %s", opts.Namespace)
	}

	if opts.PodName == "" {
		opts.PodName = "transferia-runner"
		w.logger.Info("Using default pod name: transferia-runner")
	}

	if opts.ContainerName == "" {
		opts.ContainerName = "runner"
		w.logger.Info("Using default container name: runner")
	}

	w.logger.Infof("Creating pod %s in namespace %s with image %s",
		opts.PodName, opts.Namespace, opts.Image)

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

	createdPod, err := w.client.CoreV1().Pods(opts.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		w.logger.Errorf("Failed to create pod: %v", err)
		return nil, err
	}

	w.logger.Infof("Successfully created pod %s in namespace %s",
		createdPod.Name, createdPod.Namespace)

	return createdPod, nil
}

func (w *K8sWrapper) ensureSecret(ctx context.Context, namespace string, secret *corev1.Secret) error {
	w.logger.Infof("Ensuring secret %s exists in namespace %s", secret.Name, namespace)

	// Check if secret already exists
	_, err := w.client.CoreV1().Secrets(namespace).Get(ctx, secret.Name, metav1.GetOptions{})
	if err != nil {
		// Secret doesn't exist, create it
		w.logger.Infof("Secret %s not found, creating it", secret.Name)
		_, err = w.client.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			w.logger.Errorf("Failed to create secret %s: %v", secret.Name, err)
			return xerrors.Errorf("failed to create secret %s: %w", secret.Name, err)
		}
		w.logger.Infof("Successfully created secret %s", secret.Name)
	} else {
		// Secret exists, update it
		w.logger.Infof("Secret %s exists, updating it", secret.Name)
		_, err = w.client.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
		if err != nil {
			w.logger.Errorf("Failed to update secret %s: %v", secret.Name, err)
			return xerrors.Errorf("failed to update secret %s: %w", secret.Name, err)
		}
		w.logger.Infof("Successfully updated secret %s", secret.Name)
	}

	return nil
}

func (w *K8sWrapper) Run(ctx context.Context, opts ContainerOpts) (stdout io.ReadCloser, stderr io.ReadCloser, err error) {
	w.logger.Info("Running container in Kubernetes")

	// Convert options to K8s options
	k8sOpts := opts.ToK8sOpts()

	// Create secrets if needed
	if len(k8sOpts.Secrets) > 0 {
		w.logger.Infof("Creating %d secrets for pod %s", len(k8sOpts.Secrets), k8sOpts.PodName)
		for _, secret := range k8sOpts.Secrets {
			// Create the secret
			k8sSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secret.Name,
					Namespace: k8sOpts.Namespace,
				},
				Data: secret.Data,
				Type: corev1.SecretTypeOpaque,
			}

			if err := w.ensureSecret(ctx, k8sOpts.Namespace, k8sSecret); err != nil {
				w.logger.Errorf("Failed to ensure secret %s: %v", secret.Name, err)
				return nil, nil, err
			}
		}
		w.logger.Info("All secrets created successfully")
	}

	// Create the pod
	w.logger.Infof("Creating pod %s in namespace %s", k8sOpts.PodName, k8sOpts.Namespace)
	pod, err := w.createPod(ctx, k8sOpts)
	if err != nil {
		w.logger.Errorf("Failed to create pod %s: %v", k8sOpts.PodName, err)
		return nil, nil, xerrors.Errorf("failed to create pod: %w", err)
	}
	w.logger.Infof("Pod %s created successfully in namespace %s", pod.Name, pod.Namespace)

	// Channel to signal when log streaming is done
	logStreamingDone := make(chan struct{})
	podDeleted := make(chan struct{})

	// Launch a goroutine to monitor pod completion and cleanup
	go func() {
		defer close(podDeleted)
		w.logger.Infof("Starting pod monitor for %s", pod.Name)

		watchPod := func() {
			timeout := time.After(k8sOpts.Timeout)
			tick := time.NewTicker(2 * time.Second)
			defer tick.Stop()

			w.logger.Infof("Watching pod %s with timeout %v", pod.Name, k8sOpts.Timeout)

			for {
				select {
				case <-ctx.Done():
					// Context cancelled, wait for log streaming to finish before cleanup
					w.logger.Infof("Context cancelled for pod %s, preparing for cleanup", pod.Name)
					select {
					case <-logStreamingDone:
						w.logger.Info("Log streaming completed")
					case <-time.After(10 * time.Second):
						w.logger.Warn("Timed out waiting for log streaming to complete")
					}
					w.logger.Infof("Deleting pod %s after context cancellation", pod.Name)
					err := w.client.CoreV1().Pods(pod.GetNamespace()).Delete(
						context.Background(), pod.GetName(), metav1.DeleteOptions{})
					if err != nil {
						w.logger.Errorf("Failed to delete pod %s: %v", pod.Name, err)
					} else {
						w.logger.Infof("Successfully deleted pod %s", pod.Name)
					}
					return

				case <-timeout:
					// Timeout reached, wait for log streaming to finish before cleanup
					w.logger.Warnf("Timeout reached for pod %s, preparing for cleanup", pod.Name)
					select {
					case <-logStreamingDone:
						w.logger.Info("Log streaming completed")
					case <-time.After(10 * time.Second):
						w.logger.Warn("Timed out waiting for log streaming to complete")
					}
					w.logger.Infof("Deleting pod %s after timeout", pod.Name)
					err := w.client.CoreV1().Pods(pod.GetNamespace()).Delete(
						context.Background(), pod.GetName(), metav1.DeleteOptions{})
					if err != nil {
						w.logger.Errorf("Failed to delete pod %s: %v", pod.Name, err)
					} else {
						w.logger.Infof("Successfully deleted pod %s", pod.Name)
					}
					return

				case <-tick.C:
					p, err := w.client.CoreV1().Pods(pod.GetNamespace()).Get(
						ctx, pod.GetName(), metav1.GetOptions{})
					if err != nil {
						if apierrors.IsNotFound(err) {
							w.logger.Infof("Pod %s not found, assuming it was already deleted", pod.Name)
							return // Pod already deleted, nothing to do
						}
						w.logger.Warnf("Error getting pod %s status: %v", pod.Name, err)
						continue // Other error, keep monitoring
					}

					phase := p.Status.Phase
					w.logger.Infof("Pod %s current phase: %s", pod.Name, phase)

					if phase == corev1.PodSucceeded || phase == corev1.PodFailed {
						// Pod completed
						w.logger.Infof("Pod %s completed with status: %s", pod.Name, phase)

						// Wait for log streaming to finish
						w.logger.Info("Waiting for log streaming to complete before cleanup")
						select {
						case <-logStreamingDone:
							w.logger.Info("Log streaming completed")
						case <-time.After(30 * time.Second):
							w.logger.Warn("Timed out waiting for log streaming to complete")
						}

						// Then clean up
						w.logger.Infof("Deleting completed pod %s", pod.Name)
						err := w.client.CoreV1().Pods(pod.GetNamespace()).Delete(
							context.Background(), pod.GetName(), metav1.DeleteOptions{})
						if err != nil {
							w.logger.Errorf("Failed to delete pod %s: %v", pod.Name, err)
						} else {
							w.logger.Infof("Successfully deleted pod %s", pod.Name)
						}
						return
					}
				}
			}
		}
		watchPod()
	}()

	// Wait for pod to reach Running state before trying to stream logs
	w.logger.Infof("Waiting for pod %s to be ready", pod.Name)
	if err := w.waitForPodReady(ctx, pod.GetNamespace(), pod.GetName(), 30*time.Second); err != nil {
		// If pod can't get to running state, try to get logs anyway
		// but log a warning
		w.logger.Warnf("Warning: pod %s may not be ready: %v", pod.GetName(), err)
	} else {
		w.logger.Infof("Pod %s is ready for log streaming", pod.Name)
	}

	// Set up log streaming options
	logOpts := &corev1.PodLogOptions{
		Container: k8sOpts.ContainerName,
		Follow:    true,
	}

	// Get logs stream
	w.logger.Infof("Setting up log streaming for pod %s", pod.Name)
	req := w.client.CoreV1().Pods(pod.GetNamespace()).GetLogs(pod.GetName(), logOpts)
	stream, err := req.Stream(ctx)
	if err != nil {
		w.logger.Errorf("Failed to stream logs from pod %s: %v", pod.Name, err)
		// If we can't get logs, close the logStreamingDone channel to allow pod cleanup
		close(logStreamingDone)

		// Wait for pod to be deleted
		w.logger.Info("Waiting for pod deletion")
		select {
		case <-podDeleted:
			w.logger.Infof("Pod %s was deleted", pod.Name)
		case <-time.After(5 * time.Second):
			w.logger.Warn("Timed out waiting for pod deletion")
		}

		return nil, nil, xerrors.Errorf("failed to stream pod logs: %w", err)
	}
	w.logger.Infof("Successfully established log stream for pod %s", pod.Name)

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

			w.logger.Infof("Pod %s status: %s", pod.Name, pod.Status.Phase)
			// If pod is running or succeeded/failed, it's ready for logs
			if pod.Status.Phase == corev1.PodRunning ||
				pod.Status.Phase == corev1.PodSucceeded ||
				pod.Status.Phase == corev1.PodFailed {
				// Ready to collect logs
				w.logger.Infof("Pod %s is ready for log collection", pod.Name)
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
	w.logger.Info("Running container and waiting for completion")

	stdoutReader, _, err := w.Run(ctx, opts)
	if err != nil {
		w.logger.Errorf("Failed to run container: %v", err)
		return nil, nil, err
	}
	defer stdoutReader.Close()

	w.logger.Info("Container started, collecting logs")
	stdoutBuf := new(bytes.Buffer)

	bytesRead, err := io.Copy(stdoutBuf, stdoutReader)
	if err != nil && err != io.EOF {
		w.logger.Errorf("Error copying pod logs: %v", err)
		return stdoutBuf, nil, xerrors.Errorf("error copying pod logs: %w", err)
	}

	w.logger.Infof("Container execution completed, collected %d bytes of logs", bytesRead)
	return stdoutBuf, nil, nil
}

// Type returns the container backend type
func (w *K8sWrapper) Type() ContainerBackend {
	return BackendKubernetes
}
