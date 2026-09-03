package libdocker

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSimulatorUsesBuildKit(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "hive_buildkit.txt")
	if err := os.WriteFile(marker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	usesBuildKit, err := simulatorUsesBuildKit(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !usesBuildKit {
		t.Fatal("BuildKit marker was not detected")
	}
}

func TestBuildKitCommandArgsReferenceSecretEnvironment(t *testing.T) {
	t.Setenv("HIVE_GITHUB_TOKEN", "super-secret-token")
	args, err := buildKitCommandArgs(
		"unix:///var/run/docker.sock",
		"Dockerfile",
		"hive/simulators/ethereum/eels/consume-engine:latest",
		false,
		false,
		map[string]string{"branch": "forks/amsterdam"},
		map[string]string{"github_token": "HIVE_GITHUB_TOKEN"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(args, " "), "super-secret-token") {
		t.Fatal("secret value was included in docker command arguments")
	}
	if !slices.Contains(args, "id=github_token,env=HIVE_GITHUB_TOKEN") {
		t.Fatalf("docker arguments do not contain the secret environment reference: %q", args)
	}
}

func TestBuildKitCommandArgsRejectUnsetSecretEnvironment(t *testing.T) {
	t.Setenv("HIVE_MISSING_SECRET", "")
	_, err := buildKitCommandArgs("", "Dockerfile", "image", false, false, nil, map[string]string{
		"missing": "HIVE_MISSING_SECRET",
	})
	if err == nil {
		t.Fatal("buildKitCommandArgs accepted an empty secret environment variable")
	}
}
