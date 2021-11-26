package dockercredentials

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containers/image/v5/docker/reference"

	"github.com/openshift/oc/pkg/helpers/image/credentialprovider"
)

type AuthResolver struct {
	authConfigs credentialprovider.DockerConfig
}

// NewAuthResolver creates a new auth resolver that loads authFilePath file
// (defaults to a docker locations) to find a valid
// authentication for registry targets.
func NewAuthResolver(authFilePath string) (*AuthResolver, error) {
	var cfg credentialprovider.DockerConfig
	var err error

	if authFilePath != "" {
		cfg, err = credentialprovider.ReadSpecificDockerConfigJSONFile(authFilePath)
		if err != nil {
			return nil, err
		}
	} else {
		cfg = defaultClientDockerConfig()
	}

	return &AuthResolver{
		authConfigs: cfg,
	}, nil
}

// original: https://github.com/containers/image/blob/main/pkg/docker/config/config.go
// findAuthentication looks for auth of registry in path. If ref is
// not nil, then it will be taken into account when looking up the
// authentication credentials.
func (r *AuthResolver) findAuthentication(ref reference.Named, registry string) (credentialprovider.DockerConfigEntry, error) {
	// Support for different paths in auth.
	// (This is not a feature of ~/.docker/config.json; we support it even for
	// those files as an extension.)
	var keys []string
	if ref != nil {
		keys = authKeysForRef(ref)
	} else {
		keys = []string{registry}
	}

	// Repo or namespace keys are only supported as exact matches. For registry
	// keys we prefer exact matches as well.
	for _, key := range keys {
		if val, exists := r.authConfigs[key]; exists {
			return val, nil
		}
	}

	// bad luck; let's normalize the entries first
	// This primarily happens for legacyFormat, which for a time used API URLs
	// (http[s:]//…/v1/) as keys.
	// Secondarily, (docker login) accepted URLs with no normalization for
	// several years, and matched registry hostnames against that, so support
	// those entries even in non-legacyFormat ~/.docker/config.json.
	// The docker.io registry still uses the /v1/ key with a special host name,
	// so account for that as well.
	registry = normalizeRegistry(registry)
	for k, v := range r.authConfigs {
		if normalizeAuthFileKey(k) == registry {
			return v, nil
		}
	}

	return credentialprovider.DockerConfigEntry{}, nil
}

// authKeysForRef returns the valid paths for a provided reference. For example,
// when given a reference "quay.io/repo/ns/image:tag", then it would return
// - quay.io/repo/ns/image
// - quay.io/repo/ns
// - quay.io/repo
// - quay.io
func authKeysForRef(ref reference.Named) (res []string) {
	name := ref.Name()

	for {
		res = append(res, name)

		lastSlash := strings.LastIndex(name, "/")
		if lastSlash == -1 {
			break
		}
		name = name[:lastSlash]
	}

	return res
}

// normalizeAuthFileKey takes a key, converts it to a host name and normalizes
// the resulting registry.
func normalizeAuthFileKey(key string) string {
	stripped := strings.TrimPrefix(key, "http://")
	stripped = strings.TrimPrefix(stripped, "https://")

	if stripped != key {
		stripped = strings.SplitN(stripped, "/", 2)[0]
	}

	return normalizeRegistry(stripped)
}

// normalizeRegistry converts the provided registry if a known docker.io host
// is provided.
func normalizeRegistry(registry string) string {
	switch registry {
	case "registry-1.docker.io", "docker.io":
		return "index.docker.io"
	}
	return registry
}

// defaultClientDockerConfig returns the credentials that the docker command line client would
// return.
func defaultClientDockerConfig() credentialprovider.DockerConfig {
	// support the modern config file $HOME/.docker/config.json
	if cfg, err := credentialprovider.ReadDockerConfigJSONFile(defaultPathsForCredentials()); err == nil {
		return cfg
	}
	// support the legacy config file $HOME/.dockercfg
	if cfg, err := credentialprovider.ReadDockercfgFile(defaultPathsForLegacyCredentials()); err == nil {
		return cfg
	}
	return credentialprovider.DockerConfig{}
}

// defaultPathsForCredentials returns the correct search directories for a docker config
//  file
func defaultPathsForCredentials() []string {
	if runtime.GOOS == "windows" { // Windows
		return []string{filepath.Join(os.Getenv("USERPROFILE"), ".docker")}
	}
	return []string{filepath.Join(os.Getenv("HOME"), ".docker")}
}

// defaultPathsForCredentials returns the correct search directories for a docker config
//  file
func defaultPathsForLegacyCredentials() []string {
	if runtime.GOOS == "windows" { // Windows
		return []string{os.Getenv("USERPROFILE")}
	}
	return []string{os.Getenv("HOME")}
}