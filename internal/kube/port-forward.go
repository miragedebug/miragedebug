package kube

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	spdy2 "k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"github.com/miragedebug/miragedebug/pkg/log"
)

type PodPortForwarder struct {
	stopCh     chan struct{}
	readyCh    chan struct{}
	restConfig *rest.Config
	namespace  string
	podName    string
	localPort  int32
	remotePort int32
	pf         *portforward.PortForwarder
	started    bool
}

func NewPodPortForwarder(restConfig *rest.Config, namespace string, podName string, localPort, remotePort int32) *PodPortForwarder {
	return &PodPortForwarder{
		stopCh:     make(chan struct{}),
		readyCh:    make(chan struct{}),
		restConfig: restConfig,
		namespace:  namespace,
		podName:    podName,
		localPort:  localPort,
		remotePort: remotePort,
	}
}

func (p *PodPortForwarder) PodName() string {
	return p.podName
}

func (p *PodPortForwarder) Start() error {
	if p.started {
		return nil
	}
	go func() {
		for {
			select {
			case <-p.stopCh:
				if p.pf != nil {
					p.pf.Close()
					p.pf = nil
				}
				return
			default:
				stop := make(chan struct{}, 1)
				readyCh := make(chan struct{}, 1)
				pf, err := newPortForward(context.Background(), p.restConfig, p.namespace, p.podName, p.localPort, p.remotePort, stop, readyCh)
				if err != nil {
					fmt.Printf("failed to create port-forward: %v\n", err)
					<-time.After(time.Second * 5)
					continue
				}
				p.pf = pf
				p.started = true
				log.Debugf("forward port %d to %s/%s port %d starting", p.localPort, p.namespace, p.podName, p.remotePort)
				result := make(chan error)
				go func() {
					err := p.pf.ForwardPorts()
					result <- err
				}()
				select {
				case err := <-result:
					if err != nil {
						fmt.Printf("failed to forward port: %v\n", err)
					}
					p.started = false
					<-time.After(time.Second * 3)
					continue
				case <-p.stopCh:
					close(stop)
					p.started = false
					return
				}
			}
		}
	}()
	return nil
}

func (p *PodPortForwarder) Stop() {
	close(p.stopCh)
}

func newPortForward(ctx context.Context, kubeconfig *restclient.Config, namespace string, podName string, localPort, remotePort int32, stop, ready chan struct{}) (*portforward.PortForwarder, error) {
	clientset, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}
	req := clientset.CoreV1().RESTClient().Post().Resource("pods").Namespace(namespace).Name(podName).SubResource("portforward")
	serverURL := req.URL()
	tlsConfig, err := rest.TLSConfigFor(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed getting TLS config: %w", err)
	}
	if tlsConfig == nil && kubeconfig.Transport != nil {
		// If using a custom transport, skip server verification on the upgrade.
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}
	var upgrader *spdy2.SpdyRoundTripper
	if kubeconfig.Proxy != nil {
		upgrader = spdy2.NewRoundTripperWithProxy(tlsConfig, kubeconfig.Proxy)
	} else {
		upgrader = spdy2.NewRoundTripper(tlsConfig)
	}
	roundTripper, err := rest.HTTPWrappersForConfig(kubeconfig, upgrader)

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, serverURL)

	fw, err := portforward.NewOnAddresses(dialer,
		[]string{"0.0.0.0"},
		[]string{fmt.Sprintf("%d:%d", localPort, remotePort)},
		stop,
		ready,
		io.Discard,
		os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed create port-forward: %v", err)
	}
	return fw, nil
}
