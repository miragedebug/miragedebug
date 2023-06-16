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
	c := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new project",
		RunE: func(cmd *cobra.Command, args []string) error {
			config.SetKubeconfig(kubeconfig)
			conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("did not connect: %v", err)
			}
			defer conn.Close()
			cfg, err := clientcmd.BuildConfigFromFlags("", config.GetKubeconfig())
			if err != nil {
				panic(err)
			}
			kubeClient := kubernetes.NewForConfigOrDie(cfg)
			c := app.NewAppManagementClient(conn)
			if err := promptToCreateApp(c, kubeClient); err != nil {
				log.Fatalf("create app failed: %v", err)
			}
			return nil
		},
	}
	c.PersistentFlags().StringVarP(&kubeconfig, "kubeconfig", "k", "~/.kube/config", "Kubeconfig file path.")
	c.PersistentFlags().StringVarP(&serverAddr, "server", "s", "127.0.0.1:38081", "Server grpc address")

	return c
}

type initAnswer struct {
	Name         string
	Language     string
	Namespace    string
	IDE          string
	Workdir      string
	AppEntry     string
	BuildCommand string
	BuildOutput  string
	RunArgs      string
	WorkloadType string
	Workload     string
	Container    string
	Config       string
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
			TargetArch:    app.ArchType_AMD64,
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
}

func promptToCreateApp(appClient app.AppManagementClient, kubeClient kubernetes.Interface) error {
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
						if p == "vendor" {
							return filepath.SkipDir
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
									return fmt.Sprintf("GOOS=linux GOARCH=amd64 go build -o /tmp/%s %s", a.Name, a.AppEntry)
								}
								return fmt.Sprintf("GOOS=linux GOARCH=amd64 go build -o /tmp/%s ./", a.Name)
							case app.ProgramType_RUST.String():
								return "cargo build"
							default:
								return ""
							}
						}(),
					},
				}
			},
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
		},
		{
			question: func(a *initAnswer) *survey.Question {
				app_ := a.toApp()
				bs, _ := yaml.Marshal(app_)
				return &survey.Question{
					Name: "config",
					Prompt: &survey.Editor{
						Message:       "Confirm you config:",
						Default:       string(bs),
						AppendDefault: true,
					},
				}
			},
		},
	}
	answers := initAnswer{}
	for _, q := range qs {
		question := q.question(&answers)
		if question == nil {
			continue
		}
		if err := survey.Ask([]*survey.Question{question}, &answers); err != nil {
			return err
		}
	}
	app_ := &app.App{}
	if err := yaml.Unmarshal([]byte(answers.Config), app_); err != nil {
		return fmt.Errorf("unmarshal config error: %v", err)
	}
	a, err := appClient.CreateApp(context.Background(), app_)
	if err != nil {
		return err
	}
	log.Infof("create app %s success: %v", a.Name, a)
	return err
}
