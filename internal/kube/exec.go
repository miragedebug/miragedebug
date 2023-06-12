package kube

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/kebe7jun/miragedebug/pkg/log"
)

func ExecutePodCmd(ctx context.Context, config *restclient.Config, namespace string, podName string, container string, command string, stdin io.Reader, logOut bool) (io.Reader, io.Reader, error) {
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
		Command:   strings.Fields(command),
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

	out := bytes.NewBuffer(nil)
	errOut := bytes.NewBuffer(nil)
	result := make(chan error)
	end := make(chan struct{})
	if logOut {
		go func() {
			outBuf := bytes.NewBuffer(nil)
			for {
				select {
				case <-end:
					return
				default:
					b, err := out.ReadByte()
					if err != nil {
						return
					}
					outBuf.WriteByte(b)
					if b == '\n' {
						log.Infof("pod %s/%s execute out: %s", namespace, podName, outBuf.String())
						outBuf.Reset()
					}
				}
			}
		}()
	}
	go func() {
		err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdin:  stdin,
			Stdout: out,
			Stderr: errOut,
			Tty:    false,
		})
		if err != nil {
			result <- fmt.Errorf("error in StreamWithContext: %v", err)
		} else {
			result <- nil
		}
	}()
	close(end)
	return out, errOut, <-result
}
