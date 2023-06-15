package kube

import (
	"bytes"
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/miragedebug/miragedebug/pkg/log"
)

func ExecutePodCmd(ctx context.Context, config *restclient.Config, namespace string, podName string, container string, command string, stdin io.Reader) ([]byte, []byte, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating clientset: %v", err)
	}
	req := clientset.CoreV1().RESTClient().Post().Resource("pods").Name(podName).
		Namespace(namespace).SubResource("exec")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, nil, fmt.Errorf("error adding to scheme: %v", err)
	}
	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   []string{"sh", "-c", command},
		Container: container,
		Stdin: func() bool {
			if stdin != nil {
				return true
			}
			return false
		}(),
		Stdout: true,
		Stderr: true,
		TTY:    false,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return nil, nil, fmt.Errorf("error while creating Executor: %v", err)
	}

	log.Debugf("executing pod %s/%s command: %s", namespace, podName, command)
	out := bytes.NewBuffer(nil)
	errOut := bytes.NewBuffer(nil)
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: out,
		Stderr: errOut,
		Tty:    false,
	})
	return out.Bytes(), errOut.Bytes(), err
}
