package kube

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	restclient "k8s.io/client-go/rest"

	"github.com/kebe7jun/miragedebug/pkg/log"
)

func makeTar(srcPath string, rename string, writer io.Writer) error {
	// 创建一个新的 tar.Writer
	tarWriter := tar.NewWriter(writer)
	defer tarWriter.Close()

	// 获取源路径的信息
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	// 判断是否为文件或目录
	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(srcPath)
	}

	// 递归地遍历源路径
	return filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 创建一个新的 tar.Header
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}
		if rename != "" {
			header.Name = rename
		}

		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, srcPath))
		}

		// 在 tar 包中写入 header
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// 如果是文件，再写入文件的内容
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(tarWriter, file)
		}

		return err
	})
}

func CopyLocalFileToPod(ctx context.Context, config *restclient.Config, namespace string, podName string, container string, localFile string, rename string, remotePath string) error {
	buf := bytes.NewBuffer(nil)
	err := makeTar(localFile, rename, buf)
	if err != nil {
		return err
	}
	out, errOut, err := ExecutePodCmd(ctx, config, namespace, podName, container, "tar -xf - -C "+remotePath, buf, false)
	// out, errOut, err := ExecutePodCmd(ctx, config, namespace, podName, container, "cat ", buf)
	log.Debugf("copy file %s to %s/%s out: %s, errOut: %s, err: %v", localFile, namespace, podName, out, errOut, err)
	if err != nil {
		return err
	}
	return nil
}
