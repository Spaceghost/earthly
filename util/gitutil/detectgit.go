package gitutil

import (
	"context"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/earthly/earthly/domain"
	"github.com/pkg/errors"
)

var (
	// ErrNoGitBinary is an error returned when no git binary is found.
	ErrNoGitBinary = errors.New("No git binary found")
	// ErrNotAGitDir is an error returned when a given directory is not a git dir.
	ErrNotAGitDir = errors.New("Not a git directory")
	// ErrCouldNotDetectRemote is an error returned when git remote could not be detected or parsed.
	ErrCouldNotDetectRemote = errors.New("Could not auto-detect or parse Git remote URL")
	// ErrCouldNotDetectGitHash is an error returned when git hash could not be detected.
	ErrCouldNotDetectGitHash = errors.New("Could not auto-detect or parse Git hash")
	// ErrCouldNotDetectGitShortHash is an error returned when git short hash could not be detected.
	ErrCouldNotDetectGitShortHash = errors.New("Could not auto-detect or parse Git short hash")
	// ErrCouldNotDetectGitBranch is an error returned when git branch could not be detected.
	ErrCouldNotDetectGitBranch = errors.New("Could not auto-detect or parse Git branch")
)

// GitMetadata is a collection of git information about a certain directory.
type GitMetadata struct {
	BaseDir   string
	RelDir    string
	RemoteURL string
	GitURL    string
	Hash      string
	ShortHash string
	Branch    []string
	Tags      []string
	Timestamp string
}

// Metadata performs git metadata detection on the provided directory.
func Metadata(ctx context.Context, dir string) (*GitMetadata, error) {
	err := detectGitBinary(ctx)
	if err != nil {
		return nil, err
	}
	err = detectIsGitDir(ctx, dir)
	if err != nil {
		return nil, err
	}
	baseDir, err := detectGitBaseDir(ctx, dir)
	if err != nil {
		return nil, err
	}
	var retErr error
	remoteURL, err := detectGitRemoteURL(ctx, dir)
	if err != nil {
		retErr = err
		// Keep going.
	}
	var gitURL string
	if remoteURL != "" {
		gitURL, err = ParseGitRemoteURL(remoteURL)
		if err != nil {
			return nil, err
		}
	}
	hash, err := detectGitHash(ctx, dir)
	if err != nil {
		retErr = err
		// Keep going.
	}
	shortHash, err := detectGitShortHash(ctx, dir)
	if err != nil {
		retErr = err
		// Keep going.
	}
	branch, err := detectGitBranch(ctx, dir)
	if err != nil {
		retErr = err
		// Keep going.
	}
	tags, err := detectGitTags(ctx, dir)
	if err != nil {
		// Most likely no tags. Keep going.
		tags = nil
	}
	timestamp, err := detectGitTimestamp(ctx, dir)
	if err != nil {
		retErr = err
		// Keep going.
	}

	relDir, isRel, err := gitRelDir(baseDir, dir)
	if err != nil {
		return nil, errors.Wrapf(err, "get rel dir for %s when base git path is %s", dir, baseDir)
	}
	if !isRel {
		return nil, errors.New("unexpected non-relative path within git dir")
	}

	return &GitMetadata{
		BaseDir:   filepath.ToSlash(baseDir),
		RelDir:    filepath.ToSlash(relDir),
		RemoteURL: remoteURL,
		GitURL:    gitURL,
		Hash:      hash,
		ShortHash: shortHash,
		Branch:    branch,
		Tags:      tags,
		Timestamp: timestamp,
	}, retErr
}

// Clone returns a copy of the GitMetadata object.
func (gm *GitMetadata) Clone() *GitMetadata {
	return &GitMetadata{
		BaseDir: gm.BaseDir,
		RelDir:  gm.RelDir,
		GitURL:  gm.GitURL,
		Hash:    gm.Hash,
		Branch:  gm.Branch,
		Tags:    gm.Tags,
	}
}

func detectIsGitDir(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "git", "status")
	cmd.Dir = dir
	_, err := cmd.Output()
	if err != nil {
		return ErrNotAGitDir
	}
	return nil
}

// ParseGitRemoteURL converts a gitURL like user@host.com:path/to.git or https://host.com/path/to.git to host.com/path/to
func ParseGitRemoteURL(gitURL string) (string, error) {
	s := gitURL

	// remove transport
	parts := strings.SplitN(gitURL, "://", 2)
	if len(parts) == 2 {
		s = parts[1]
	}

	// remove user
	parts = strings.SplitN(s, "@", 2)
	if len(parts) == 2 {
		s = parts[1]
	}

	s = strings.Replace(s, ":", "/", 1)
	s = strings.TrimSuffix(s, ".git")
	return s, nil
}

func detectGitRemoteURL(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "config", "--get", "remote.origin.url")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(
			ErrCouldNotDetectRemote, "returned error %s: %s", err.Error(), string(out))
	}
	outStr := string(out)
	if outStr == "" {
		return "", errors.Wrapf(ErrCouldNotDetectRemote, "no remote origin url output")
	}
	return strings.SplitN(outStr, "\n", 2)[0], nil
}

func detectGitBaseDir(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "detect git directory")
	}
	outStr := string(out)
	if outStr == "" {
		return "", errors.New("No output returned for git base dir")
	}
	return strings.SplitN(outStr, "\n", 2)[0], nil
}

func detectGitHash(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(ErrCouldNotDetectGitHash, "returned error %s: %s", err.Error(), string(out))
	}
	outStr := string(out)
	if outStr == "" {
		return "", errors.Wrapf(ErrCouldNotDetectGitHash, "no remote origin url output")
	}
	return strings.SplitN(outStr, "\n", 2)[0], nil
}

func detectGitShortHash(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--short=8", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(ErrCouldNotDetectGitShortHash, "returned error %s: %s", err.Error(), string(out))
	}
	outStr := string(out)
	if outStr == "" {
		return "", errors.Wrapf(ErrCouldNotDetectGitShortHash, "no remote origin url output")
	}
	return strings.SplitN(outStr, "\n", 2)[0], nil
}

func detectGitBranch(ctx context.Context, dir string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrapf(ErrCouldNotDetectGitBranch, "returned error %s: %s", err.Error(), string(out))
	}
	outStr := string(out)
	if outStr != "" {
		return strings.Split(outStr, "\n"), nil
	}
	return nil, nil
}

func detectGitTags(ctx context.Context, dir string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "describe", "--exact-match", "--tags")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "detect git current tags")
	}
	outStr := string(out)
	if outStr != "" {
		return strings.Split(outStr, "\n"), nil
	}
	return nil, nil
}

func detectGitTimestamp(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "log", "-1", "--format=%ct")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "0", nil
	}
	outStr := string(out)
	if outStr == "" {
		return "0", nil
	}
	return strings.SplitN(outStr, "\n", 2)[0], nil
}

func gitRelDir(basePath string, path string) (string, bool, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", false, errors.Wrapf(err, "get abs path for %s", path)
	}
	absPath2, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", false, errors.Wrapf(err, "eval symlinks for %s", absPath)
	}
	if !filepath.IsAbs(basePath) {
		return "", false, errors.Errorf("git base path %s is not absolute", basePath)
	}
	basePathSlash := filepath.ToSlash(basePath)
	pathSlash := filepath.ToSlash(absPath2)
	basePathParts := strings.Split(basePathSlash, "/")
	pathParts := strings.Split(pathSlash, "/")
	if len(pathParts) < len(basePathParts) {
		return "", false, nil
	}
	for index := range basePathParts {
		if basePathParts[index] != pathParts[index] {
			return "", false, nil
		}
	}
	relPath := strings.Join(pathParts[len(basePathParts):], "/")
	if relPath == "" {
		return ".", true, nil
	}
	return filepath.FromSlash(relPath), true, nil
}

// ReferenceWithGitMeta applies git metadata to the target naming.
func ReferenceWithGitMeta(ref domain.Reference, gitMeta *GitMetadata) domain.Reference {
	if gitMeta == nil || gitMeta.GitURL == "" {
		return ref
	}
	gitURL := gitMeta.GitURL
	if gitMeta.RelDir != "" {
		gitURL = path.Join(gitURL, gitMeta.RelDir)
	}
	tag := ref.GetTag()
	localPath := ref.GetLocalPath()
	name := ref.GetName()
	importRef := ref.GetImportRef()

	if tag == "" {
		if len(gitMeta.Tags) > 0 {
			tag = gitMeta.Tags[0]
		} else if len(gitMeta.Branch) > 0 {
			tag = gitMeta.Branch[0]
		} else {
			tag = gitMeta.Hash
		}
	}

	switch ref.(type) {
	case domain.Target:
		return domain.Target{
			GitURL:    gitURL,
			Tag:       tag,
			LocalPath: localPath,
			ImportRef: importRef,
			Target:    name,
		}
	case domain.Command:
		return domain.Command{
			GitURL:    gitURL,
			Tag:       tag,
			LocalPath: localPath,
			ImportRef: importRef,
			Command:   name,
		}
	default:
		panic("not supported for this type")
	}
}
