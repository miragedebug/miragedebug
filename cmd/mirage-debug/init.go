package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	"github.com/miragedebug/miragedebug/api/app"
	"github.com/miragedebug/miragedebug/config"
	"github.com/miragedebug/miragedebug/pkg/log"
)

func initCmd() *cobra.Command {
	serverAddr := ""
	kubeconfig := ""
	answers := &initAnswer{}
	c := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new project",
		RunE: func(cmd *cobra.Command, args []string) error {
			config.SetKubeconfig(kubeconfig)
			conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("did not connect: %v", err)
				return nil
			}
			defer conn.Close()
			cfg, err := clientcmd.BuildConfigFromFlags("", config.GetKubeconfig())
			if err != nil {
				panic(err)
			}
			kubeClient := kubernetes.NewForConfigOrDie(cfg)
			c := app.NewAppManagementClient(conn)
			if err := promptToCreateApp(c, kubeClient, answers); err != nil {
				log.Fatalf("create app failed: %v", err)
			}
			return nil
		},
	}
	c.PersistentFlags().StringVarP(&kubeconfig, "kubeconfig", "k", "~/.kube/config", "Kubeconfig file path.")
	c.PersistentFlags().StringVarP(&serverAddr, "server", "s", "127.0.0.1:38081", "Server grpc address")

	c.PersistentFlags().StringVarP(&answers.Name, "name", "", "", "App name")
	c.PersistentFlags().StringVarP(&answers.Language, "language", "", "", "Programming language")
	c.PersistentFlags().StringVarP(&answers.Namespace, "namespace", "", "", "App running namespace")
	c.PersistentFlags().StringVarP(&answers.WorkloadType, "workload-type", "", "", "App workload type")
	c.PersistentFlags().StringVarP(&answers.Workload, "workload", "", "", "App workload name")
	c.PersistentFlags().StringVarP(&answers.Container, "container", "", "", "App Container")
	c.PersistentFlags().StringVarP(&answers.RemoteArch, "remote-arch", "", "", "App Arch")
	c.PersistentFlags().StringVarP(&answers.IDE, "ide", "", "", "IDE type")
	c.PersistentFlags().StringVarP(&answers.Workdir, "workdir", "", "", "Source code path")
	c.PersistentFlags().StringVarP(&answers.AppEntry, "app-entry", "", "", "Entry path of the app, relative to the workdir")
	c.PersistentFlags().StringVarP(&answers.BuildCommand, "build-command", "", "", "How to build the app")
	c.PersistentFlags().StringVarP(&answers.BuildOutput, "build-output", "", "", "Build output path")
	c.PersistentFlags().StringVarP(&answers.RunArgs, "run-args", "", "", "Run args")

	return c
}

type initAnswer struct {
	Name         string
	Language     string
	Namespace    string
	WorkloadType string
	Workload     string
	Container    string
	RemoteArch   string
	IDE          string
	Workdir      string
	AppEntry     string
	BuildCommand string
	BuildOutput  string
	RunArgs      string
	CustomConfig bool
	Config       string
}

func (answers *initAnswer) archType() app.ArchType {
	if strings.EqualFold(answers.RemoteArch, "arm64") {
		return app.ArchType_ARM64
	}
	return app.ArchType_AMD64
}

func (answers *initAnswer) toApp() *app.App {
	return &app.App{
		Name:        answers.Name,
		ProgramType: app.ProgramType(app.ProgramType_value[answers.Language]),
		RemoteRuntime: &app.RemoteRuntime{
			Namespace:     answers.Namespace,
			WorkloadType:  app.WorkloadType(app.WorkloadType_value[answers.WorkloadType]),
			WorkloadName:  answers.Workload,
			ContainerName: answers.Container,
			TargetArch:    answers.archType(),
		},
		LocalConfig: &app.LocalConfig{
			IdeType:            app.IDEType(app.IDEType_value[answers.IDE]),
			WorkingDir:         answers.Workdir,
			AppArgs:            answers.RunArgs,
			AppEntryPath:       answers.AppEntry,
			CustomBuildCommand: answers.BuildCommand,
			BuildOutput:        answers.BuildOutput,
			DebugToolBuilder: &app.DebugToolBuilder{
				Type: app.DebugToolType_LOCAL,
			},
		},
	}
}

type questionWrap struct {
	question func(a *initAnswer) *survey.Question
	bind     *string
}

func promptToCreateApp(appClient app.AppManagementClient, kubeClient kubernetes.Interface, answers *initAnswer) error {
	workloadPodTemplateMap := map[string]corev1.PodTemplateSpec{}
	qs := []*questionWrap{
		{
			question: func(a *initAnswer) *survey.Question {
				return &survey.Question{
					Name:   "name",
					Prompt: &survey.Input{Message: "What is app name?"},
					Validate: func(ans interface{}) error {
						if ans.(string) == "" {
							return fmt.Errorf("app name can not be empty")
						}
						if a, _ := appClient.GetApp(context.Background(), &app.SingleAppRequest{Name: ans.(string)}); a != nil {
							return fmt.Errorf("app %s already exists", ans.(string))
						}
						return nil
					},
				}
			},
			bind: &answers.Name,
		},
		{
			question: func(a *initAnswer) *survey.Question {
				return &survey.Question{
					Name: "language",
					Prompt: &survey.Select{
						Message: "Choose a programing language:",
						Options: []string{app.ProgramType_GO.String(), app.ProgramType_RUST.String()},
						Default: app.ProgramType_GO.String(),
					},
				}
			},
			bind: &answers.Language,
		},
		{
			question: func(a *initAnswer) *survey.Question {
				return &survey.Question{
					Name: "remoteArch",
					Prompt: &survey.Select{
						Message: "What kind of arch:",
						Options: []string{app.ArchType_AMD64.String(), app.ArchType_ARM64.String()},
						Default: app.ArchType_AMD64.String(),
					},
				}
			},
			bind: &answers.RemoteArch,
		},
		{
			question: func(a *initAnswer) *survey.Question {
				return &survey.Question{
					Name: "namespace",
					Prompt: &survey.Select{
						Message: "Choose a namespace you want to debug:",
						Options: func() []string {
							nsList, _ := kubeClient.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
							return lo.Map(nsList.Items, func(item corev1.Namespace, index int) string {
								return item.Name
							})
						}(),
						Default:  "default",
						PageSize: 10,
					},
				}
			},
			bind: &answers.Namespace,
		},
		{
			question: func(a *initAnswer) *survey.Question {
				return &survey.Question{
					Name: "workloadType",
					Prompt: &survey.Select{
						Message: "What kind of workload:",
						Options: []string{app.WorkloadType_DEPLOYMENT.String(), app.WorkloadType_DAEMONSET.String()},
						Default: app.WorkloadType_DEPLOYMENT.String(),
					},
				}
			},
			bind: &answers.WorkloadType,
		},
		{
			question: func(a *initAnswer) *survey.Question {
				return &survey.Question{
					Name: "workload",
					Prompt: &survey.Select{
						Message: "Please choose a workload:",
						Options: func() []string {
							switch a.WorkloadType {
							case app.WorkloadType_DEPLOYMENT.String():
								depList, _ := kubeClient.AppsV1().Deployments(a.Namespace).List(context.Background(), metav1.ListOptions{})
								return lo.Map(depList.Items, func(item appsv1.Deployment, index int) string {
									workloadPodTemplateMap[item.Name] = item.Spec.Template
									return item.Name
								})
							case app.WorkloadType_DAEMONSET.String():
								depList, _ := kubeClient.AppsV1().DaemonSets(a.Namespace).List(context.Background(), metav1.ListOptions{})
								return lo.Map(depList.Items, func(item appsv1.DaemonSet, index int) string {
									workloadPodTemplateMap[item.Name] = item.Spec.Template
									return item.Name
								})
							}
							return nil
						}(),
						PageSize: 10,
					},
				}
			},
			bind: &answers.Workload,
		},
		{
			question: func(a *initAnswer) *survey.Question {
				if len(workloadPodTemplateMap[a.Workload].Spec.Containers) == 1 {
					a.Container = workloadPodTemplateMap[a.Workload].Spec.Containers[0].Name
					return nil
				}
				return &survey.Question{
					Name: "container",
					Prompt: &survey.Select{
						Message: "Please choose a container:",
						Options: lo.Map(workloadPodTemplateMap[a.Workload].Spec.Containers, func(item corev1.Container, index int) string {
							return item.Name
						}),
					},
				}
			},
			bind: &answers.Container,
		},
		{
			question: func(a *initAnswer) *survey.Question {
				if os.Getenv("TERM_PROGRAM") == "vscode" {
					a.IDE = app.IDEType_VS_CODE.String()
				} else if os.Getenv("__CFBundleIdentifier") == "com.jetbrains.goland" {
					a.IDE = app.IDEType_GOLAND.String()
				} else if os.Getenv("__CFBundleIdentifier") == "com.jetbrains.CLion" {
					a.IDE = app.IDEType_CLION.String()
				}
				if a.IDE != "" {
					fmt.Printf("detected your IDE: %s\n", a.IDE)
					return nil
				}
				return &survey.Question{
					Name: "ide",
					Prompt: &survey.Select{
						Message: "Choose a IDE type:",
						Options: []string{
							app.IDEType_VS_CODE.String(),
							app.IDEType_GOLAND.String(),
							app.IDEType_CLION.String()},
					},
				}
			},
			bind: &answers.IDE,
		},
		{
			question: func(a *initAnswer) *survey.Question {
				return &survey.Question{
					Name: "workdir",
					Prompt: &survey.Input{
						Message: "Your project workdir: ",
						Default: func() string {
							dir, _ := os.Getwd()
							return dir
						}(),
					},
					Validate: func(ans interface{}) error {
						if _, err := os.Stat(ans.(string)); os.IsNotExist(err) {
							return errors.New("directory not exist")
						}
						return nil
					},
				}
			},
			bind: &answers.Workdir,
		},
		{
			question: func(a *initAnswer) *survey.Question {
				if a.Language == app.ProgramType_GO.String() {
					paths := []string{""}
					filepath.Walk(a.Workdir, func(p string, info fs.FileInfo, err error) error {
						if info.IsDir() {
							return nil
						}
						p = strings.TrimPrefix(p, a.Workdir)
						if info.Name() == "main.go" {
							paths = append(paths, "."+path.Dir(p))
						}
						return nil
					})
					var p survey.Prompt
					p = &survey.Input{
						Message: "Your app entry path: ",
					}
					if len(paths) > 0 {
						p = &survey.Select{
							Message: "Your app entry path: ",
							Options: paths,
						}
					}
					return &survey.Question{
						Name:   "appEntry",
						Prompt: p,
					}
				}
				return nil
			},
			bind: &answers.AppEntry,
		},
		{
			question: func(a *initAnswer) *survey.Question {
				return &survey.Question{
					Name: "buildCommand",
					Prompt: &survey.Input{
						Message: "How to build your project: ",
						Default: func() string {
							switch a.Language {
							case app.ProgramType_GO.String():
								// todo support arm64
								if a.AppEntry != "" {
									a.BuildOutput = "/tmp/" + a.Name
									return fmt.Sprintf("GOOS=linux GOARCH=%s go build -o /tmp/%s %s", app.ToSystemArch(a.archType()), a.Name, a.AppEntry)
								}
								return fmt.Sprintf("GOOS=linux GOARCH=%s go build -o /tmp/%s ./", app.ToSystemArch(a.archType()), a.Name)
							case app.ProgramType_RUST.String():
								return "cargo build"
							default:
								return ""
							}
						}(),
					},
				}
			},
			bind: &answers.BuildCommand,
		},
		{
			question: func(a *initAnswer) *survey.Question {
				if a.BuildOutput != "" {
					return nil
				}
				return &survey.Question{
					Name: "buildOutput",
					Prompt: &survey.Input{
						Message: "Build output binary path: ",
						Default: func() string {
							switch a.Language {
							case app.ProgramType_GO.String():
								return "/tmp/" + a.Name
							default:
								return ""
							}
						}(),
					},
				}
			},
			bind: &answers.BuildOutput,
		},
		{
			question: func(a *initAnswer) *survey.Question {
				var args []string
				if c, ok := lo.Find(workloadPodTemplateMap[a.Workload].Spec.Containers, func(item corev1.Container) bool {
					return item.Name == a.Container
				}); ok {
					args = c.Args
				}
				return &survey.Question{
					Name: "runArgs",
					Prompt: &survey.Input{
						Message: "App running args(eg. --port 8080):",
						Default: strings.Join(args, " "),
					},
				}
			},
			bind: &answers.RunArgs,
		},
		{
			question: func(a *initAnswer) *survey.Question {
				app_ := a.toApp()
				bs, _ := yaml.Marshal(app_)
				return &survey.Question{
					Name: "customConfig",
					Prompt: &survey.Confirm{
						Message: "App Config yaml: \n" + string(bs) + "\nDo you want to customize it?",
						Default: false,
					},
				}
			},
		},
		{
			question: func(a *initAnswer) *survey.Question {
				if !a.CustomConfig {
					return nil
				}
				app_ := a.toApp()
				bs, _ := yaml.Marshal(app_)
				return &survey.Question{
					Name: "config",
					Prompt: &survey.Editor{
						Default:       string(bs),
						HideDefault:   true,
						AppendDefault: true,
					},
				}
			},
		},
	}
	for _, q := range qs {
		if !(q.bind == nil || *q.bind == "") {
			continue
		}
		question := q.question(answers)
		if question == nil {
			continue
		}
		if err := survey.Ask([]*survey.Question{question}, answers); err != nil {
			return err
		}
	}
	app_ := answers.toApp()
	if answers.CustomConfig {
		if err := yaml.Unmarshal([]byte(answers.Config), app_); err != nil {
			return fmt.Errorf("unmarshal config error: %v", err)
		}
	}
	a, err := appClient.CreateApp(context.Background(), app_)
	if err != nil {
		return err
	}
	fmt.Printf("Created app %s successfully.\n Run `%s config %s` in %s to config your IDE!\n", a.Name, os.Args[0], a.Name, answers.Workdir)
	return err
}
