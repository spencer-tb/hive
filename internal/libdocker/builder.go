package libdocker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ethereum/hive/internal/libhive"
	docker "github.com/fsouza/go-dockerclient"
)

// Builder takes care of building docker images.
type Builder struct {
	client        *docker.Client
	config        *Config
	logger        *slog.Logger
	authenticator *docker.AuthConfigurations
}

func NewBuilder(client *docker.Client, cfg *Config, auth *docker.AuthConfigurations) *Builder {
	b := &Builder{
		client:        client,
		config:        cfg,
		logger:        cfg.Logger,
		authenticator: auth,
	}
	if b.logger == nil {
		b.logger = slog.Default()
	}
	return b
}

// BuildClientImage builds a docker image of the given client.
func (b *Builder) BuildClientImage(ctx context.Context, client libhive.ClientDesignator) (string, error) {
	dir := b.config.Inventory.ClientDirectory(client)
	tag := fmt.Sprintf("hive/clients/%s:latest", client.Name())
	dockerFile := client.Dockerfile()
	err := b.buildImage(ctx, dir, dockerFile, tag, client.BuildArgs, nil, false)
	return tag, err
}

// BuildSimulatorImage builds a docker image of a simulator.
func (b *Builder) BuildSimulatorImage(ctx context.Context, name string, buildArgs, buildSecrets map[string]string) (string, error) {
	dir := b.config.Inventory.SimulatorDirectory(name)
	buildContextPath := dir
	buildDockerfile := "Dockerfile"
	useBuildKit, err := simulatorUsesBuildKit(dir)
	if err != nil {
		return "", err
	}

	// build context dir of simulator can be overridden with "hive_context.txt" file containing the desired build path
	if contextPathBytes, err := os.ReadFile(filepath.Join(filepath.FromSlash(dir), "hive_context.txt")); err == nil {
		buildContextPath = filepath.Join(dir, strings.TrimSpace(string(contextPathBytes)))
		if strings.HasPrefix(buildContextPath, "../") {
			return "", fmt.Errorf("cannot access build directory outside of Hive root: %q", buildContextPath)
		}
		if p, err := filepath.Rel(buildContextPath, filepath.Join(filepath.FromSlash(dir), "Dockerfile")); err != nil {
			return "", fmt.Errorf("failed to derive relative simulator Dockerfile path: %v", err)
		} else {
			buildDockerfile = p
		}
	}
	tag := fmt.Sprintf("hive/simulators/%s:latest", name)
	err = b.buildImage(ctx, buildContextPath, buildDockerfile, tag, buildArgs, buildSecrets, useBuildKit)
	return tag, err
}

// BuildImage creates a container by archiving the given file system,
// which must contain a file called "Dockerfile".
func (b *Builder) BuildImage(ctx context.Context, name string, fsys fs.FS) error {
	opts := b.buildConfig(ctx, name)
	pipeR, pipeW := io.Pipe()
	opts.InputStream = pipeR
	go b.archiveFS(ctx, pipeW, fsys)

	b.logger.Info("building image", "image", name, "nocache", opts.NoCache, "pull", b.config.PullEnabled)
	if err := b.client.BuildImage(opts); err != nil {
		if imageAlreadyExists(err) {
			b.logger.Info("image already exists", "image", name)
		} else {
			b.logger.Error("image build failed", "image", name, "err", err)
		}
		return err
	}
	return nil
}

// imageAlreadyExists reports whether a build failed only because a concurrent
// hive process already produced the identical image, a benign cross-process
// race when multiple simulations share one Docker daemon.
func imageAlreadyExists(err error) bool {
	ok, _ := regexp.MatchString("\\bAlreadyExists: ", err.Error())
	return ok
}

func (b *Builder) buildConfig(ctx context.Context, name string) docker.BuildImageOptions {
	nocache := false
	if b.config.NoCachePattern != nil {
		nocache = b.config.NoCachePattern.MatchString(name)
	}
	opts := docker.BuildImageOptions{
		Context:      ctx,
		Name:         name,
		OutputStream: io.Discard,
		NoCache:      nocache,
		Pull:         b.config.PullEnabled,
	}
	if b.authenticator != nil {
		opts.AuthConfigs = *b.authenticator
	}
	if b.config.BuildOutput != nil {
		opts.OutputStream = b.config.BuildOutput
	}
	return opts
}

func (b *Builder) archiveFS(ctx context.Context, out io.WriteCloser, fsys fs.FS) error {
	defer out.Close()

	w := tar.NewWriter(out)
	err := fs.WalkDir(fsys, ".", func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		// Write header.
		if e.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s: symlinks are not supported in BuildImage", path)
		}
		info, err := e.Info()
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		hdr.Name = path
		if err := w.WriteHeader(hdr); err != nil {
			return err
		}

		// Write file content.
		if e.Type().IsRegular() {
			file, err := fsys.Open(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, file); err != nil {
				file.Close()
				return err
			}
			file.Close()
		}

		return nil
	})

	if err != nil {
		return err
	}

	// TODO: errors
	w.Flush()
	w.Close()
	return nil
}

// ReadFile returns the content of a file in the given image. To do so, it creates a
// temporary container, downloads the file from it and destroys the container.
func (b *Builder) ReadFile(ctx context.Context, image, path string) ([]byte, error) {
	// Create the temporary container and ensure it's cleaned up.
	opt := docker.CreateContainerOptions{
		Context: ctx,
		Config:  &docker.Config{Image: image},
	}
	cont, err := b.client.CreateContainer(opt)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := b.client.RemoveContainer(docker.RemoveContainerOptions{ID: cont.ID, Force: true}); err != nil {
			b.logger.Error("can't remove temporary container", "id", cont.ID[:8], "err", err)
		}
	}()

	// Download a tarball of the file from the container.
	download := new(bytes.Buffer)
	dlopt := docker.DownloadFromContainerOptions{
		Path:         path,
		OutputStream: download,
		Context:      ctx,
	}
	if err := b.client.DownloadFromContainer(cont.ID, dlopt); err != nil {
		return nil, err
	}
	in := tar.NewReader(download)
	for {
		// Fetch the next file header from the archive.
		header, err := in.Next()
		if err != nil {
			return nil, err
		}
		// If it's the file we're looking for, save its contents.
		if header.Name == path[1:] {
			content := new(bytes.Buffer)
			if _, err := io.Copy(content, in); err != nil {
				return nil, err
			}
			return content.Bytes(), nil
		}
	}
}

// buildImage builds a single docker image from the specified context.
// branch specifies a build argument to use a specific base image branch or github source branch.
func (b *Builder) buildImage(ctx context.Context, contextDir, dockerFile, imageTag string, buildArgs, buildSecrets map[string]string, useBuildKit bool) error {
	logger := b.logger.With("image", imageTag)
	buildContext, err := filepath.Abs(contextDir)
	if err != nil {
		logger.Error("can't find path to context directory", "err", err)
		return err
	}
	if len(buildSecrets) > 0 && !useBuildKit {
		return fmt.Errorf("simulator build secrets require a hive_buildkit.txt marker in %s", contextDir)
	}
	if useBuildKit {
		return b.buildImageWithBuildKit(ctx, buildContext, dockerFile, imageTag, buildArgs, buildSecrets)
	}

	opts := b.buildConfig(ctx, imageTag)
	opts.ContextDir = buildContext
	opts.Dockerfile = dockerFile
	logctx := []interface{}{"dir", contextDir, "nocache", opts.NoCache, "pull", opts.Pull}
	if len(buildArgs) > 0 {
		args := convertBuildArgs(buildArgs)
		for _, arg := range args {
			logctx = append(logctx, arg.Name, arg.Value)
		}
		opts.BuildArgs = args
	}

	logger.Info("building image", logctx...)
	if err := b.client.BuildImage(opts); err != nil {
		logger.Error("image build failed", "err", err)
		return err
	}
	return nil
}

// simulatorUsesBuildKit reports whether a simulator opts into the BuildKit
// build path. A marker keeps this choice independent of the Dockerfile frontend
// directive, which can require downloading a frontend image.
func simulatorUsesBuildKit(simulatorDir string) (bool, error) {
	_, err := os.Stat(filepath.Join(simulatorDir, "hive_buildkit.txt"))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("can't inspect BuildKit marker: %w", err)
}

// buildImageWithBuildKit invokes the Docker CLI because the Docker Engine build
// API used by go-dockerclient cannot attach the session required to transfer
// BuildKit secrets. Secret values remain in the referenced environment
// variables; only their IDs and environment variable names appear in argv.
func (b *Builder) buildImageWithBuildKit(ctx context.Context, contextDir, dockerFile, imageTag string, buildArgs, buildSecrets map[string]string) error {
	opts := b.buildConfig(ctx, imageTag)
	args, err := buildKitCommandArgs(b.client.Endpoint(), dockerFile, imageTag, opts.NoCache, opts.Pull, buildArgs, buildSecrets)
	if err != nil {
		return err
	}
	logctx := []interface{}{"dir", contextDir, "nocache", opts.NoCache, "pull", opts.Pull, "builder", "buildkit"}
	for _, arg := range convertBuildArgs(buildArgs) {
		logctx = append(logctx, arg.Name, arg.Value)
	}
	secretIDs := make([]string, 0, len(buildSecrets))
	for id := range buildSecrets {
		secretIDs = append(secretIDs, id)
	}
	slices.Sort(secretIDs)
	for _, id := range secretIDs {
		envName := buildSecrets[id]
		logctx = append(logctx, "secret", id+"=env:"+envName)
	}

	logger := b.logger.With("image", imageTag)
	logger.Info("building image", logctx...)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = contextDir
	cmd.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
	cmd.Stdout = opts.OutputStream
	cmd.Stderr = opts.OutputStream
	if err := cmd.Run(); err != nil {
		logger.Error("image build failed", "err", err)
		return fmt.Errorf("BuildKit image build failed: %w", err)
	}
	return nil
}

func buildKitCommandArgs(endpoint, dockerFile, imageTag string, noCache, pull bool, buildArgs, buildSecrets map[string]string) ([]string, error) {
	args := make([]string, 0, 8+2*(len(buildArgs)+len(buildSecrets)))
	if endpoint != "" {
		args = append(args, "--host", endpoint)
	}
	args = append(args, "build", "--file", dockerFile, "--tag", imageTag)
	if noCache {
		args = append(args, "--no-cache")
	}
	if pull {
		args = append(args, "--pull")
	}
	for _, arg := range convertBuildArgs(buildArgs) {
		args = append(args, "--build-arg", arg.Name+"="+arg.Value)
	}
	secretIDs := make([]string, 0, len(buildSecrets))
	for id := range buildSecrets {
		secretIDs = append(secretIDs, id)
	}
	slices.Sort(secretIDs)
	for _, id := range secretIDs {
		envName := buildSecrets[id]
		if value, ok := os.LookupEnv(envName); !ok || value == "" {
			return nil, fmt.Errorf("build secret %q references unset or empty environment variable %q", id, envName)
		}
		args = append(args, "--secret", "id="+id+",env="+envName)
	}
	return append(args, "."), nil
}

func convertBuildArgs(m map[string]string) []docker.BuildArg {
	args := make([]docker.BuildArg, 0, len(m))
	for key, value := range m {
		args = append(args, docker.BuildArg{Name: key, Value: value})
	}
	slices.SortFunc(args, func(a, b docker.BuildArg) int {
		return strings.Compare(a.Name, b.Name)
	})
	return args
}
