package main

import (
	"context"
	"fmt"
	"os"
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

func promptToCreateApp(appClient app.AppManagementClient, kubeClient kubernetes.Interface) error {
	answers := struct {
		Name         string
		Language     string
		Namespace    string
		IDE          string
		Workdir      string
		BuildCommand string
		BuildOutput  string
		RunArgs      string
	}{}
	questions := []*survey.Question{
		{
			Name:   "name",
			Prompt: &survey.Input{Message: "What is app name?"},
			Validate: func(ans interface{}) error {
				if ans.(string) == "" {
					return fmt.Errorf("app name can not be empty")
				}
				if a, _ := appClient.GetApp(context.Background(), &app.SingleAppRequest{Name: ans.(string)}); a != nil {
					return fmt.Errorf("app name already exists")
				}
				return nil
			},
		},
		{
			Name: "language",
			Prompt: &survey.Select{
				Message: "Choose a programing language:",
				Options: []string{app.ProgramType_GO.String(), app.ProgramType_RUST.String()},
				Default: app.ProgramType_GO.String(),
			},
		},
		{
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
		},
	}
	err := survey.Ask(questions, &answers)
	if err != nil {
		return err
	}
	workloadArgsMap := map[string][]string{}
	promptWorkload := &survey.Select{
		Message: "Please choose an workload:",
		Options: func() []string {
			depList, _ := kubeClient.AppsV1().Deployments(answers.Namespace).List(context.Background(), metav1.ListOptions{})
			return lo.Map(depList.Items, func(item appsv1.Deployment, index int) string {
				workloadArgsMap[item.Name] = item.Spec.Template.Spec.Containers[0].Args
				return item.Name
			})
		}(),
		PageSize: 10,
	}
	workload := ""
	if err := survey.AskOne(promptWorkload, &workload); err != nil {
		return err
	}
	localConfigQuestions := []*survey.Question{
		{
			Name: "ide",
			Prompt: &survey.Select{
				Message: "Choose a IDE type:",
				Options: []string{app.IDEType_VS_CODE.String(), app.IDEType_GOLAND.String(), app.IDEType_CLION.String()},
			},
		},
		{
			Name: "workdir",
			Prompt: &survey.Input{
				Message: "Your project workdir: ",
				Default: func() string {
					dir, _ := os.Getwd()
					return dir
				}(),
			},
		},
		{
			Name: "buildCommand",
			Prompt: &survey.Input{
				Message: "How to build your project: ",
				Default: func() string {
					switch answers.Language {
					case app.ProgramType_GO.String():
						return "GOOS=linux GOARCH=amd64 go build -o ./bin/main ./"
					case app.ProgramType_RUST.String():
						return "cargo build --target x86_64-unknown-linux-gnu"
					default:
						return ""
					}
				}(),
			},
		},
		{
			Name: "buildOutput",
			Prompt: &survey.Input{
				Message: "Build output binary path: ",
				Default: func() string {
					switch answers.Language {
					case app.ProgramType_GO.String():
						return "./bin/main"
					default:
						return ""
					}
				}(),
			},
		},
		{
			Name: "runArgs",
			Prompt: &survey.Input{
				Message: "App running args(eg. --port 8080:",
				Default: strings.Join(workloadArgsMap[workload], " "),
			},
		},
	}
	err = survey.Ask(localConfigQuestions, &answers)
	if err != nil {
		return err
	}
	app_ := &app.App{
		Name:        answers.Name,
		ProgramType: app.ProgramType(app.ProgramType_value[answers.Language]),
		RemoteRuntime: &app.RemoteRuntime{
			Namespace:    answers.Namespace,
			WorkloadType: app.WorkloadType_DEPLOYMENT,
			WorkloadName: workload,
			TargetArch:   app.ArchType_AMD64,
		},
		LocalConfig: &app.LocalConfig{
			IdeType:            app.IDEType(app.IDEType_value[answers.IDE]),
			WorkingDir:         answers.Workdir,
			AppArgs:            answers.RunArgs,
			CustomBuildCommand: answers.BuildCommand,
			BuildOutput:        answers.BuildOutput,
			DebugToolBuilder: &app.DebugToolBuilder{
				Type: app.DebugToolType_LOCAL,
			},
		},
	}
	a, err := appClient.CreateApp(context.Background(), app_)
	if err != nil {
		return err
	}
	log.Infof("create app %s success: %v", a.Name, a)
	return err
}
