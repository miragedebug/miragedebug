package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	"github.com/miragedebug/miragedebug/api/app"
	"github.com/miragedebug/miragedebug/config"
	"github.com/miragedebug/miragedebug/internal/kube"
	langadaptors "github.com/miragedebug/miragedebug/internal/lang-adaptors"
	"github.com/miragedebug/miragedebug/internal/lang-adaptors/golang"
	"github.com/miragedebug/miragedebug/internal/lang-adaptors/rust"
	debug_tools "github.com/miragedebug/miragedebug/internal/local/debug-tools"
	"github.com/miragedebug/miragedebug/pkg/log"
)

const configDebugLabel = "miragedebug.io/debug"
const appFileSuffix = ".yaml"

func appsDir() string {
	return path.Join(config.GetConfigRootPath(), "apps")
}

func appFile(name string) string {
	return path.Join(appsDir(), name+appFileSuffix)
}

func readAppFromFile(f string) (*app.App, error) {
	bs, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}
	a := app.App{}
	err = yaml.Unmarshal(bs, &a)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func readApp(name string) (*app.App, error) {
	f := appFile(name)
	return readAppFromFile(f)
}

type appDebugConfig struct {
	port             int32
	podPortForwarder *kube.PodPortForwarder
}

type appManagement struct {
	app.UnimplementedAppManagementServer
	inited         bool
	rwlock         sync.RWMutex
	kubeconfig     *rest.Config
	kubeclient     kubernetes.Interface
	debugConfigMap map[string]appDebugConfig
}

func (a *appManagement) init() {
	a.rwlock.Lock()
	defer a.rwlock.Unlock()
	if a.inited {
		return
	}
	os.MkdirAll(appsDir(), 0755)
	a.inited = true
	a.debugConfigMap = make(map[string]appDebugConfig)
	cfg, err := clientcmd.BuildConfigFromFlags("", config.GetKubeconfig())
	if err != nil {
		panic(err)
	}
	a.kubeconfig = cfg
	a.kubeclient = kubernetes.NewForConfigOrDie(cfg)
}

func (a *appManagement) save(app_ *app.App) error {
	bs, err := yaml.Marshal(app_)
	if err != nil {
		return err
	}
	return os.WriteFile(appFile(app_.Name), bs, 0755)
}

func (a *appManagement) getAppRelatedWorkloadTemplate(ctx context.Context, app_ *app.App) (*corev1.PodTemplateSpec, error) {
	switch app_.RemoteRuntime.WorkloadType {
	case app.WorkloadType_DEPLOYMENT:
		dep, err := a.kubeclient.AppsV1().Deployments(app_.RemoteRuntime.Namespace).Get(ctx, app_.RemoteRuntime.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return &dep.Spec.Template, nil
	case app.WorkloadType_DAEMONSET:
		dep, err := a.kubeclient.AppsV1().DaemonSets(app_.RemoteRuntime.Namespace).Get(ctx, app_.RemoteRuntime.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return &dep.Spec.Template, nil
	}
	return nil, fmt.Errorf("unsupported workload type %s", app_.RemoteRuntime.WorkloadType.String())
}

func (a *appManagement) getAppRelatedPod(ctx context.Context, app_ *app.App) (*corev1.Pod, error) {
	ls := fmt.Sprintf("%s=%s", configDebugLabel, app_.Name)
	if app_.RemoteConfig.GetNoModifyConfig() {
		tmpl, err := a.getAppRelatedWorkloadTemplate(ctx, app_)
		if err != nil {
			return nil, err
		}
		ls = labels.SelectorFromSet(tmpl.Labels).String()
	}
	podList, err := a.kubeclient.CoreV1().Pods(app_.RemoteRuntime.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: ls,
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(podList.Items, func(i, j int) bool {
		return podList.Items[i].CreationTimestamp.After(podList.Items[j].CreationTimestamp.Time)
	})
	if len(podList.Items) == 0 {
		return nil, fmt.Errorf("no pod found")
	}
	return &podList.Items[0], nil
}

func (a *appManagement) setAppRelatedWorkloadTemplate(ctx context.Context, app_ *app.App, tmpl corev1.PodTemplateSpec) error {
	switch app_.RemoteRuntime.WorkloadType {
	case app.WorkloadType_DEPLOYMENT:
		dep, err := a.kubeclient.AppsV1().Deployments(app_.RemoteRuntime.Namespace).Get(ctx, app_.RemoteRuntime.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		dep.Spec.Template = tmpl
		_, err = a.kubeclient.AppsV1().Deployments(app_.RemoteRuntime.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
		return err
	case app.WorkloadType_DAEMONSET:
		dep, err := a.kubeclient.AppsV1().DaemonSets(app_.RemoteRuntime.Namespace).Get(ctx, app_.RemoteRuntime.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		dep.Spec.Template = tmpl
		_, err = a.kubeclient.AppsV1().DaemonSets(app_.RemoteRuntime.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
		return err
	}
	return fmt.Errorf("unsupported workload type %s", app_.RemoteRuntime.WorkloadType.String())
}

func (a *appManagement) getApp(name string) (*app.App, bool) {
	app_, err := readApp(name)
	if err != nil {
		log.Errorf("read app %s error: %v", name, err)
	}
	return app_, err == nil
}

func (a *appManagement) GetServerInfo(ctx context.Context, empty *app.Empty) (*app.ServerInfo, error) {
	return &app.ServerInfo{
		Pid: int32(os.Getpid()),
	}, nil
}

func (a *appManagement) ListApps(ctx context.Context, empty *app.Empty) (*app.AppList, error) {
	entries, err := os.ReadDir(appsDir())
	if err != nil {
		return nil, err
	}
	appList := &app.AppList{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), appFileSuffix) {
			continue
		}
		f := path.Join(appsDir(), e.Name())
		app_, err := readAppFromFile(f)
		if err != nil {
			log.Errorf("read app from %s error: %v", f, err)
			continue
		}
		appList.Apps = append(appList.Apps, app_)
	}
	return appList, nil
}

func (a *appManagement) CreateApp(ctx context.Context, app_ *app.App) (*app.App, error) {
	if _, ok := a.getApp(app_.Name); ok {
		return nil, fmt.Errorf("app %s already exists", app_.Name)
	}
	if app_.RemoteRuntime == nil {
		return nil, fmt.Errorf("remote runtime is required")
	}
	if app_.RemoteRuntime.Namespace == "" ||
		app_.RemoteRuntime.WorkloadType == app.WorkloadType_WORKLOAD_TYPE_UNSPECIFIED ||
		app_.RemoteRuntime.WorkloadName == "" {
		return nil, fmt.Errorf("remote runtime namespace, workload type and workload name are required")
	}
	if app_.RemoteRuntime.TargetArch == app.ArchType_ARCH_TYPE_UNSPECIFIED {
		app_.RemoteRuntime.TargetArch = app.ArchType_AMD64
	}
	if err := a.save(app_); err != nil {
		return nil, err
	}
	return app_, nil
}

func (a *appManagement) UpdateApp(ctx context.Context, request *app.App) (*app.App, error) {
	_, ok := a.getApp(request.Name)
	if !ok {
		return nil, fmt.Errorf("app %s not found", request.Name)
	}
	return request, a.save(request)
}

func (a *appManagement) DeleteApp(ctx context.Context, request *app.SingleAppRequest) (*app.App, error) {
	app_, ok := a.getApp(request.Name)
	if !ok {
		return nil, fmt.Errorf("app %s not found", request.Name)
	}
	if err := os.Remove(appFile(request.Name)); err != nil {
		return nil, err
	}
	return app_, nil
}

func (a *appManagement) GetApp(ctx context.Context, request *app.SingleAppRequest) (*app.App, error) {
	app, ok := a.getApp(request.Name)
	if !ok {
		return nil, fmt.Errorf("app %s not found", request.Name)
	}
	return app, nil
}

func (a *appManagement) GetAppStatus(ctx context.Context, request *app.SingleAppRequest) (*app.Status, error) {
	app_, ok := a.getApp(request.Name)
	if !ok {
		return nil, fmt.Errorf("app %s not found", request.Name)
	}
	tmpl, err := a.getAppRelatedWorkloadTemplate(ctx, app_)
	if err != nil {
		return nil, err
	}
	configured := tmpl.Labels[configDebugLabel] == app_.Name
	return &app.Status{
		AppName:    app_.Name,
		Configured: configured,
	}, nil
}

// InitAppRemote will do the following things:
//  1. config the workload to ready for debug.
//     a. change command and args.
//     b. change the replica to 1 and other things.
//  2. installing debug tool in container.
//  3. port-forward the remote debugging port.
func (a *appManagement) InitAppRemote(ctx context.Context, request *app.SingleAppRequest) (*app.Status, error) {
	app_, ok := a.getApp(request.Name)
	if !ok {
		return nil, fmt.Errorf("app %s not found", request.Name)
	}
	if app_.LocalConfig == nil || app_.LocalConfig.DebugToolBuilder == nil {
		return nil, fmt.Errorf("app %s is not configured local debuging config", request.Name)
	}
	if app_.RemoteConfig == nil {
		app_.RemoteConfig = &app.RemoteConfig{
			DebugToolPath:       "/tmp/debug-tool",
			RemoteAppLocation:   "/tmp",
			RemoteDebuggingPort: int32(rand.Int()%10000 + 50000),
			InitialConfig:       "",
		}
	}
	tmpl, err := a.getAppRelatedWorkloadTemplate(ctx, app_)
	if err != nil {
		return nil, err
	}
	// 1. config the workload to ready for debug.
	needUpdate := false
	calcTarget := func(tmpl *corev1.PodTemplateSpec) {
		tmpl.Labels[configDebugLabel] = app_.Name
		for i := range tmpl.Spec.Containers {
			if tmpl.Spec.Containers[i].Name == app_.RemoteRuntime.ContainerName || app_.RemoteRuntime.ContainerName == "" {
				tmpl.Spec.Containers[i].Command = []string{"/bin/sh"}
				tmpl.Spec.Containers[i].Args = []string{"-c", "touch /tmp/mirage-debug-output; tail -f /tmp/mirage-debug-output"}
				if tmpl.Spec.Containers[i].SecurityContext != nil {
					tmpl.Spec.Containers[i].SecurityContext.ReadOnlyRootFilesystem = pointer.Bool(false)
				}
				tmpl.Spec.Containers[i].ReadinessProbe = nil
				break
			}
		}
	}
	if app_.RemoteConfig.InitialConfig != "" {
		target := corev1.PodTemplateSpec{}
		_ = json.Unmarshal([]byte(app_.RemoteConfig.InitialConfig), &target)
		calcTarget(&target)
		if !reflect.DeepEqual(&target, tmpl) {
			tmpl = &target
			needUpdate = true
		}
	} else {
		bs, err := json.Marshal(tmpl)
		if err != nil {
			return nil, err
		}
		calcTarget(tmpl)
		app_.RemoteConfig.InitialConfig = string(bs)
		if err := a.save(app_); err != nil {
			return nil, err
		}
		needUpdate = true
	}
	if needUpdate && !app_.RemoteConfig.GetNoModifyConfig() {
		if err := a.setAppRelatedWorkloadTemplate(ctx, app_, *tmpl); err != nil {
			return nil, err
		}
	}
	if app_.RemoteRuntime.ContainerName == "" {
		app_.RemoteRuntime.ContainerName = tmpl.Spec.Containers[0].Name
	}
	timer := time.NewTimer(time.Second * 3)
	defer timer.Stop()
	ctx, cancel := context.WithTimeout(ctx, time.Second*60)
	defer cancel()
	// wait for the workload to be ready.
	podName := ""
	var lastErr error
loop:
	for {
		select {
		case <-timer.C:
			pod, err := a.getAppRelatedPod(ctx, app_)
			if err != nil {
				lastErr = err
				log.Errorf("get pod of app %s failed: %v", app_.Name, err)
			} else {
				if pod.Status.Phase == corev1.PodRunning {
					podName = pod.Name
					break loop
				} else {
					log.Debugf("pod %s is not running", pod.Name)
					lastErr = fmt.Errorf("pod %s is not running", pod.Name)
				}
			}
		case <-ctx.Done():
			lastErr = fmt.Errorf("wait for pod running timeout")
		}
	}
	if podName == "" {
		return nil, lastErr
	}
	// 2. installing debug tool in container.
	if err := debug_tools.InstallPodDebugTool(ctx, app_, a.kubeconfig, podName); err != nil {
		return nil, err
	}
	a.save(app_)
	// 3. port-forward the remote debugging port.
	if debugConifg, ok := a.debugConfigMap[app_.Name]; !ok {
		pf := kube.NewPodPortForwarder(a.kubeconfig, app_.RemoteRuntime.Namespace, podName, app_.RemoteConfig.RemoteDebuggingPort, app_.RemoteConfig.RemoteDebuggingPort)
		if err := pf.Start(); err != nil {
			return nil, err
		}
		a.debugConfigMap[app_.Name] = appDebugConfig{
			port:             app_.RemoteConfig.RemoteDebuggingPort,
			podPortForwarder: pf,
		}
	} else if debugConifg.port != app_.RemoteConfig.RemoteDebuggingPort ||
		debugConifg.podPortForwarder.PodName() != podName {
		debugConifg.podPortForwarder.Stop()
		pf := kube.NewPodPortForwarder(a.kubeconfig, app_.RemoteRuntime.Namespace, podName, app_.RemoteConfig.RemoteDebuggingPort, app_.RemoteConfig.RemoteDebuggingPort)
		if err := pf.Start(); err != nil {
			return nil, err
		}
		a.debugConfigMap[app_.Name] = appDebugConfig{
			port:             app_.RemoteConfig.RemoteDebuggingPort,
			podPortForwarder: pf,
		}
	}
	return &app.Status{
		AppName:    app_.Name,
		Configured: true,
		Connected:  true,
		Debugging:  false,
		Error:      "",
	}, nil
}

func (a *appManagement) StartDebugging(ctx context.Context, request *app.SingleAppRequest) (*app.Empty, error) {
	app_, ok := a.getApp(request.Name)
	if !ok {
		return nil, fmt.Errorf("app %s not found", request.Name)
	}
	pod, err := a.getAppRelatedPod(ctx, app_)
	if err != nil {
		return nil, err
	}
	binaryFile := app_.LocalConfig.BuildOutput
	if !strings.HasPrefix(binaryFile, "/") {
		binaryFile = path.Join(app_.LocalConfig.WorkingDir, app_.LocalConfig.BuildOutput)
	}
	if err := kube.CopyLocalFileToPod(ctx, a.kubeconfig, app_.RemoteRuntime.Namespace, pod.Name, app_.RemoteRuntime.ContainerName, binaryFile, "", app_.RemoteConfig.RemoteAppLocation); err != nil {
		return nil, err
	}
	var langAdaptor langadaptors.LanguageAdaptor
	switch app_.ProgramType {
	case app.ProgramType_GO:
		langAdaptor = golang.NewGolangAdaptor()
	case app.ProgramType_RUST:
		langAdaptor = rust.NewRustAdaptor()
	default:
		return nil, fmt.Errorf("unsupported program type %s", app_.ProgramType)
	}
	command, err := langAdaptor.DebugCommand(app_)
	if err != nil {
		return nil, err
	}
	kube.ExecutePodCmd(ctx, a.kubeconfig, app_.RemoteRuntime.Namespace, pod.Name, app_.RemoteRuntime.ContainerName,
		fmt.Sprintf("pkill -9 %s; pkill -9 %s", path.Base(app_.RemoteConfig.DebugToolPath), path.Base(app_.LocalConfig.BuildOutput)),
		nil)
	go func() {
		_, _, err = kube.ExecutePodCmd(context.Background(), a.kubeconfig, app_.RemoteRuntime.Namespace, pod.Name, app_.RemoteRuntime.ContainerName, fmt.Sprintf("%s 2>&1 >>/tmp/mirage-debug-output", command), nil)
		if err != nil {
			log.Errorf("start debugging failed: %v", err)
		}
	}()
	<-time.After(time.Second * 3)
	return &app.Empty{}, err
}

func (a *appManagement) RollbackApp(ctx context.Context, request *app.SingleAppRequest) (*app.Status, error) {
	app_, ok := a.getApp(request.Name)
	if !ok {
		return nil, fmt.Errorf("app %s not found", request.Name)
	}
	if app_.GetRemoteConfig().GetInitialConfig() == "" {
		return nil, fmt.Errorf("no initial config found")
	}
	target := corev1.PodTemplateSpec{}
	_ = json.Unmarshal([]byte(app_.RemoteConfig.InitialConfig), &target)
	if err := a.setAppRelatedWorkloadTemplate(ctx, app_, target); err != nil {
		return nil, err
	}
	return &app.Status{
		AppName:    app_.Name,
		Configured: false,
		Connected:  false,
		Debugging:  false,
		Error:      "",
	}, nil
}
