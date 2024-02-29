package yaml

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"go.keploy.io/server/v2/pkg/models"
	"go.uber.org/zap"
	yamlLib "gopkg.in/yaml.v3"
)

// NetworkTrafficDoc stores the request-response data of a network call (ingress or egress)
type NetworkTrafficDoc struct {
	Version models.Version `json:"version" yaml:"version"`
	Kind    models.Kind    `json:"kind" yaml:"kind"`
	Name    string         `json:"name" yaml:"name"`
	Spec    yamlLib.Node   `json:"spec" yaml:"spec"`
	Curl    string         `json:"curl" yaml:"curl,omitempty"`
}

// ctxReader wraps an io.Reader with a context for cancellation support
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (cr *ctxReader) Read(p []byte) (n int, err error) {
	select {
	case <-cr.ctx.Done():
		return 0, cr.ctx.Err()
	default:
		return cr.r.Read(p)
	}
}

// ctxWriter wraps an io.Writer with a context for cancellation support
type ctxWriter struct {
	ctx    context.Context
	writer io.Writer
}

func (cw *ctxWriter) Write(p []byte) (n int, err error) {
	for len(p) > 0 {
		select {
		case <-cw.ctx.Done():
			return n, cw.ctx.Err()
		default:
			var written int
			written, err = cw.writer.Write(p)
			n += written
			if err != nil {
				return n, err
			}
			p = p[written:]
		}
	}
	return n, nil
}

func WriteFile(ctx context.Context, logger *zap.Logger, path, fileName string, docData []byte) error {
	isFileEmpty, err := CreateYamlFile(ctx, logger, path, fileName)
	if err != nil {
		return err
	}
	data := []byte("---\n")
	if isFileEmpty {
		data = []byte{}
	}
	data = append(data, docData...)
	yamlPath := filepath.Join(path, fileName+".yaml")
	file, err := os.OpenFile(yamlPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		logger.Error("failed to open file for writing", zap.Error(err), zap.String("file", yamlPath))
		return err
	}
	defer file.Close()

	cw := &ctxWriter{
		ctx:    ctx,
		writer: file,
	}

	_, err = cw.Write(data)
	if err != nil {
		if err == ctx.Err() {
			return nil // Ignore context cancellation error
		}
		logger.Error("failed to write the yaml document", zap.Error(err), zap.String("yaml file name", fileName))
		return err
	}
	return nil
}

func ReadFile(ctx context.Context, path, name string) ([]byte, error) {
	filePath := filepath.Join(path, name+".yaml")
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read the file: %v", err)
	}
	defer file.Close()

	cr := &ctxReader{
		ctx: ctx,
		r:   file,
	}

	data, err := io.ReadAll(cr)
	if err != nil {
		if err == ctx.Err() {
			return nil, nil // Ignore context cancellation error
		}
		return nil, fmt.Errorf("failed to read the file: %v", err)
	}
	return data, nil
}

func CreateYamlFile(ctx context.Context, Logger *zap.Logger, path string, fileName string) (bool, error) {
	yamlPath, err := ValidatePath(filepath.Join(path, fileName+".yaml"))
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(yamlPath); err != nil {
		err = os.MkdirAll(filepath.Join(path), fs.ModePerm)
		if err != nil {
			Logger.Error("failed to create a directory for the yaml file", zap.Error(err), zap.String("path directory", path), zap.String("yaml", fileName))
			return false, err
		}
		file, err := os.OpenFile(yamlPath, os.O_CREATE, 0777) // Set file permissions to 777
		if err != nil {
			Logger.Error("failed to create a yaml file", zap.Error(err), zap.String("path directory", path), zap.String("yaml", fileName))
			return false, err
		}
		file.Close()
		return true, nil
	}
	return false, nil
}

func ReadSessionIndices(ctx context.Context, path string, Logger *zap.Logger) ([]string, error) {
	indices := []string{}
	dir, err := ReadDir(path, fs.FileMode(os.O_RDONLY))
	if err != nil {
		Logger.Debug("creating a folder for the keploy generated testcases", zap.Error(err))
		return indices, nil
	}

	files, err := dir.ReadDir(0)
	if err != nil {
		return indices, err
	}

	for _, v := range files {
		if v.Name() != "testReports" {
			indices = append(indices, v.Name())
		}
	}
	return indices, nil
}
