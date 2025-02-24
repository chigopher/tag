package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/chigopher/pathlib"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-playground/validator/v10"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/cobra"
)

var ErrNoNewVersion = errors.New("no new version specified")

var EXIT_CODE_NO_NEW_VERSION = 8

func versionFromFile() (string, error) {
	var versionPath *pathlib.Path
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working dir: %w", err)
	}
	dir := pathlib.NewPath(cwd)
	for {
		if len(dir.Parts()) == 1 {
			break
		}
		versionPathCandidate := dir.Join("VERSION")
		exists, err := versionPathCandidate.Exists()
		if err != nil {
			return "", fmt.Errorf("determining if VERSION file exists at %s: %w", versionPath, err)
		}
		if exists {
			versionPath = versionPathCandidate
			break
		}
		dir = dir.Parent()
	}
	if versionPath == nil {
		return "", errors.New("unable to find VERSION file in any path up to root")
	}
	fileBytes, err := versionPath.ReadFile()
	if err != nil {
		return "", fmt.Errorf("reading version file: %w", err)
	}
	firstLine := strings.Split(string(fileBytes), "\n")[0]
	return strings.TrimSuffix(firstLine, "\n"), nil

}

func NewTagCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use: "tag",
		Run: func(cmd *cobra.Command, args []string) {
			k := koanf.New(".")
			if err := k.Load(
				env.Provider(
					"TAG_",
					".",
					func(s string) string {
						return strings.Replace(strings.ToLower(strings.TrimPrefix(s, "TAG_")), "_", "-", -1)
					}),
				nil,
			); err != nil {
				handleErr(err)
			}
			if err := k.Load(posflag.Provider(cmd.PersistentFlags(), ".", k), nil); err != nil {
				handleErr(err)
			}
			version, err := versionFromFile()
			if err != nil {
				handleErr(err)
			}
			k.Set("version", version)

			tagger, err := NewTagger(k)
			if err != nil {
				handleErr(err)
			}

			requestedVersion, previousVersion, err := tagger.Tag()
			if requestedVersion != nil && previousVersion != nil {
				fmt.Fprintf(os.Stdout, "v%s,v%s", requestedVersion.String(), previousVersion.String())
			}
			if err != nil {
				if errors.Is(ErrNoNewVersion, err) {
					os.Exit(EXIT_CODE_NO_NEW_VERSION)
				}
				handleErr(err)
			}
		},
	}
	flags := cmd.PersistentFlags()
	flags.Bool("dry-run", false, "print, but do not perform, any actions")

	return cmd, nil
}

func (t *Tagger) createTag(repo *git.Repository, version string) error {
	hash, err := repo.Head()
	if err != nil {
		return fmt.Errorf("finding repo HEAD: %w", err)
	}

	if t.DryRun {
		logger.Info().Str("tag", version).Msg("would have created tag")
		return nil
	}
	majorVersion := strings.Split(version, ".")[0]
	for _, v := range []string{version, majorVersion} {
		if err := repo.DeleteTag(v); err != nil {
			logger.Info().Err(err).Str("tag", v).Msg("failed to delete tag, but probably not an issue.")
		}
		_, err = repo.CreateTag(v, hash.Hash(), &git.CreateTagOptions{
			Tagger: &object.Signature{
				Name:  "Landon Clipp",
				Email: "11232769+LandonTClipp@users.noreply.github.com",
				When:  time.Now(),
			},
			Message: v,
		})
		if err != nil {
			return fmt.Errorf("creating git tag: %w", err)
		}
	}

	logger.Info().Str("tag", version).Msg("tag successfully created")
	return nil
}

func (t *Tagger) largestTagSemver(repo *git.Repository, major uint64) (*semver.Version, error) {
	largestTag, err := semver.NewVersion("v0.0.0")
	if err != nil {
		return nil, fmt.Errorf("creating new semver version: %w", err)
	}

	iter, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("getting repo tags: %w", err)
	}
	if err := iter.ForEach(func(ref *plumbing.Reference) error {
		var versionString string
		tag, err := repo.TagObject(ref.Hash())
		switch err {
		case nil:
			versionString = tag.Name
		case plumbing.ErrObjectNotFound:
			versionString = ref.Name().Short()
		default:
			// Some other error
			return fmt.Errorf("getting tag from hash: %w", err)
		}

		versionParts := strings.Split(versionString, ".")
		if len(versionParts) < 3 {
			// This is not a full version tag, so ignore it
			return nil
		}

		version, err := semver.NewVersion(versionString)
		if err != nil {
			return fmt.Errorf("creating new semver version: %w", err)
		}
		if version.GreaterThan(largestTag) && version.Major() == major {
			largestTag = version
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return largestTag, nil
}

func NewTagger(k *koanf.Koanf) (*Tagger, error) {
	t := &Tagger{}
	if err := k.Unmarshal("", t); err != nil {
		return nil, fmt.Errorf("unmarshalling tag config: %w", err)
	}
	if err := validator.New(
		validator.WithRequiredStructEnabled(),
	).Struct(t); err != nil {
		return nil, fmt.Errorf("validating struct: %w", err)
	}
	return t, nil
}

type Tagger struct {
	VersionFile string `koanf:"version-file"`
	DryRun      bool   `koanf:"dry-run"`
	Version     string `koanf:"version" validate:"required"`
}

func (t *Tagger) Tag() (requestedVersion *semver.Version, previousVersion *semver.Version, err error) {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return nil, nil, fmt.Errorf("opening git repo: %w", err)
	}

	requestedVersion, err = semver.NewVersion(t.Version)
	if err != nil {
		logger.Err(err).Str("requested-version", string(t.Version)).Msg("error when constructing semver from version config")
		return requestedVersion, nil, fmt.Errorf("creating new server version: %w", err)
	}

	previousVersion, err = t.largestTagSemver(repo, requestedVersion.Major())
	if err != nil {
		return requestedVersion, previousVersion, err
	}
	logger := logger.With().
		Stringer("previous-version", previousVersion).Logger()

	logger.Info().Msg("found largest semver tag")

	logger = logger.With().
		Stringer("requested-version", requestedVersion).
		Logger()
	if !requestedVersion.GreaterThan(previousVersion) {
		logger.Info().
			Msg("VERSION is not greater than latest git tag, nothing to do.")
		return requestedVersion, previousVersion, ErrNoNewVersion
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return requestedVersion, previousVersion, fmt.Errorf("getting repo worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return requestedVersion, previousVersion, fmt.Errorf("getting worktree status: %w", err)
	}
	if !status.IsClean() {
		logger.Error().Msg("git is in a dirty state, can't tag.")
		fmt.Println(status.String())
		return requestedVersion, previousVersion, errors.New("dirty git state")
	}

	if err := t.createTag(repo, fmt.Sprintf("v%s", requestedVersion.String())); err != nil {
		return requestedVersion, previousVersion, err
	}
	logger.Info().Msg("created new tag. Push to origin still required.")

	return requestedVersion, previousVersion, nil
}
