package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"

	"github.com/kebe7jun/miragedebug/api/app"
	"github.com/kebe7jun/miragedebug/config"
	"github.com/kebe7jun/miragedebug/internal/kube"
	"github.com/kebe7jun/miragedebug/internal/local/debug-tools/godlv"
	"github.com/kebe7jun/miragedebug/pkg/log"
)

const appsJsonFileName = "apps.json"
const configDebugLabel = "miragedebug.io/debug"

type appDebugConfig struct {
	port             int32
	podPortForwarder *kube.PodPortForwarder
}

type appManagement struct {
	app.UnimplementedAppManagementServer
	apps           []*app.App
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
	os.MkdirAll(config.GetConfigRootPath(), 0755)
	bs, err := os.ReadFile(path.Join(config.GetConfigRootPath(), appsJsonFileName))
	if err != nil {
		bs = []byte("[]")
		if err = os.WriteFile(path.Join(config.GetConfigRootPath(), appsJsonFileName), bs, 0644); err != nil {
			panic(err)
		}
	}
	apps := make([]*app.App, 0)
	if err = json.Unmarshal(bs, &apps); err != nil {
		panic(err)
	}
	a.apps = apps
	a.inited = true
	a.debugConfigMap = make(map[string]appDebugConfig)
	cfg, err := clientcmd.BuildConfigFromFlags("", config.GetKubeconfig())
	if err != nil {
		panic(err)
	}
	a.kubeconfig = cfg
	a.kubeclient = kubernetes.NewForConfigOrDie(cfg)
}

func (a *appManagement) addAndSave(app *app.App) error {
	as := append(a.apps, app)
	if err := a.save(as); err != nil {
		return err
	}
	a.rwlock.Lock()
	defer a.rwlock.Unlock()
	a.apps = as
	return nil
}

func (a *appManagement) updateAndSave(app_ *app.App) error {
	a.rwlock.Lock()
	lo.ForEach(a.apps, func(v *app.App, i int) {
		if v.Name == app_.Name {
			a.apps[i] = app_
		}
	})
	a.rwlock.Unlock()
	if err := a.save(a.apps); err != nil {
		return err
	}
	return nil
}

func (a *appManagement) save(apps []*app.App) error {
	a.rwlock.RLock()
	defer a.rwlock.RUnlock()
	bs, err := json.MarshalIndent(apps, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(config.GetConfigRootPath(), appsJsonFileName), bs, 0644)
}

func (a *appManagement) saveSelf() error {
	a.rwlock.RLock()
	defer a.rwlock.RUnlock()
	bs, err := json.Marshal(a.apps)
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(config.GetConfigRootPath(), appsJsonFileName), bs, 0644)
}

func (a *appManagement) getAppRelatedWorkloadTemplate(ctx context.Context, app_ *app.App) (*corev1.PodTemplateSpec, error) {
	switch app_.RemoteRuntime.WorkloadType {
	case app.WorkloadType_DEPLOYMENT:
		dep, err := a.kubeclient.AppsV1().Deployments(app_.RemoteRuntime.Namespace).Get(ctx, app_.RemoteRuntime.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return &dep.Spec.Template, nil
	}
	return nil, fmt.Errorf("unsupported workload type %s", app_.RemoteRuntime.WorkloadType.String())
}

func (a *appManagement) getAppRelatedPod(ctx context.Context, app_ *app.App) (*corev1.Pod, error) {
	podList, err := a.kubeclient.CoreV1().Pods(app_.RemoteRuntime.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", configDebugLabel, app_.Name),
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
	}
	return fmt.Errorf("unsupported workload type %s", app_.RemoteRuntime.WorkloadType.String())
}

func (a *appManagement) getApp(name string) (*app.App, bool) {
	a.rwlock.RLock()
	defer a.rwlock.RUnlock()
	return lo.Find(a.apps, func(item *app.App) bool {
		return item.Name == name
	})
}

func (a *appManagement) ListApps(ctx context.Context, empty *app.Empty) (*app.AppList, error) {
	a.rwlock.RLock()
	defer a.rwlock.RUnlock()
	return &app.AppList{Apps: a.apps}, nil
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
	// clean some fields
	app_.LocalConfig = nil
	if err := a.addAndSave(app_); err != nil {
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
				tmpl.Spec.Containers[i].Args = []string{"-c", "sleep 1000000000"}
				if tmpl.Spec.Containers[i].SecurityContext != nil {
					tmpl.Spec.Containers[i].SecurityContext.ReadOnlyRootFilesystem = pointer.Bool(false)
				}
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
		if err := a.saveSelf(); err != nil {
			return nil, err
		}
		needUpdate = true
	}
	if needUpdate {
		tmpl.Labels[configDebugLabel] = app_.Name
		for i := range tmpl.Spec.Containers {
			if tmpl.Spec.Containers[i].Name == app_.RemoteRuntime.ContainerName || app_.RemoteRuntime.ContainerName == "" {
				tmpl.Spec.Containers[i].Command = []string{"/bin/sh"}
				tmpl.Spec.Containers[i].Args = []string{"-c", "sleep 1000000000"}
				break
			}
		}
		if err := a.setAppRelatedWorkloadTemplate(ctx, app_, *tmpl); err != nil {
			return nil, err
		}
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
	switch app_.LocalConfig.DebugToolBuilder.Type {
	case app.DebugToolType_LOCAL:
		switch app_.ProgramType {
		case app.ProgramType_GO:
			p, err := godlv.InitOrLoadDLV(app_.RemoteRuntime.TargetArch, app_.LocalConfig.DebugToolBuilder.BuildCommands)
			if err != nil {
				return nil, err
			}
			err = kube.CopyLocalFileToPod(ctx, a.kubeconfig, app_.RemoteRuntime.Namespace, podName, app_.RemoteRuntime.ContainerName, p, path.Base(app_.RemoteConfig.DebugToolPath), path.Dir(app_.RemoteConfig.DebugToolPath))
			if err != nil {
				return nil, err
			}
		}
	default:
		// todo implement me
		return nil, fmt.Errorf("not implemented")
	}
	a.updateAndSave(app_)
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
	} else if debugConifg.port != app_.RemoteConfig.RemoteDebuggingPort {
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
	if err := kube.CopyLocalFileToPod(ctx, a.kubeconfig, app_.RemoteRuntime.Namespace, pod.Name, app_.RemoteRuntime.ContainerName, app_.LocalConfig.BuildOutput, "", app_.RemoteConfig.RemoteAppLocation); err != nil {
		return nil, err
	}
	command := fmt.Sprintf("%s --listen=:%d --headless=true --api-version=2 --accept-multiclient --check-go-version=false exec -- %s %s",
		app_.RemoteConfig.DebugToolPath,
		app_.RemoteConfig.RemoteDebuggingPort,
		path.Join(app_.RemoteConfig.RemoteAppLocation, path.Base(app_.LocalConfig.BuildOutput)),
		app_.LocalConfig.AppArgs,
	)
	kube.ExecutePodCmd(ctx, a.kubeconfig, app_.RemoteRuntime.Namespace, pod.Name, app_.RemoteRuntime.ContainerName, "pkill -9 "+path.Base(app_.RemoteConfig.DebugToolPath), nil, false)
	go func() {
		_, _, err = kube.ExecutePodCmd(context.Background(), a.kubeconfig, app_.RemoteRuntime.Namespace, pod.Name, app_.RemoteRuntime.ContainerName, command, nil, true)
	}()
	<-time.After(time.Second * 3)
	return &app.Empty{}, err
}

func (a *appManagement) RollbackApp(ctx context.Context, request *app.SingleAppRequest) (*app.Status, error) {
	// TODO implement me
	panic("implement me")
}

func (a *appManagement) mustEmbedUnimplementedAppManagementServer() {
	// TODO implement me
	panic("implement me")
}
