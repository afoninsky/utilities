package semantic

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

type SemanticVersion uint8

const initialVersion = "0.0.0"
const treeIsDirtyFlag = "dirty"
const breakingChangeText = "BREAKING CHANGE:"

const (
	VersionInvalid SemanticVersion = iota
	VersionNone
	VersionPatch
	VersionMinor
	VersionMajor
)

type Commit struct {
	Version SemanticVersion
	Hash    string
	Type    string
	Scope   string
	Message string
}

// ReleaseInfo contains information about commit considered as release
type ReleaseInfo struct {
	// string representation of latest released version
	LatestVersion string
	// string representation of next possible version
	// will be empty if semantic commits does not exist
	// possible tag of current commit
	CurrentTag  string
	NextVersion string
	// List of commits since current version
	NextCommits []Commit
}

type Repository struct {
	git       *git.Repository
	typeMajor []string
	typeMinor []string
	typePatch []string
}

func New(repoPath string) (*Repository, error) {
	git, err := openRepoRecursive(repoPath)
	if err != nil {
		return nil, err
	}

	r := &Repository{}
	r.git = git

	r.typeMajor = []string{"break"}
	r.typeMinor = []string{"feat"}
	r.typePatch = []string{"fix", "ref", "perf"}

	return r, nil
}

// Info returns semantic information about selected repository
func (r *Repository) Info() (ReleaseInfo, error) {
	i := ReleaseInfo{}

	//
	lastVersion, lastVersionCommit, err := r.getLatestVersion()
	if err != nil {
		return i, err
	}
	i.LatestVersion = lastVersion

	//
	nextVersion, commits, err := r.getNextVersion(lastVersion, lastVersionCommit)
	if err != nil {
		return i, err
	}
	i.NextVersion = nextVersion
	i.NextCommits = commits

	//
	headCommit, isDirty, err := r.getTreeStatus()
	if err != nil {
		return i, err
	}
	var hash string
	if lastVersionCommit != headCommit {
		hash = headCommit
	}
	i.CurrentTag = createTag(lastVersion, hash, isDirty)

	return i, nil
}

// PushExperimental pushes code to remote repository using provided credentials
// can be usefull in some CI cases
func (r *Repository) PushExperimental(user, password, key string) error {

	// check remote URL to ensure what auth should be applied
	remote, err := r.git.Remote("origin")
	if err != nil {
		return err
	}
	urls := remote.Config().URLs
	url := urls[0]

	fmt.Println(url)

	var auth transport.AuthMethod
	switch {
	case strings.HasPrefix(url, "git"):
		sshAuth, err := ssh.NewPublicKeysFromFile(
			user,
			sanitizeSSHKeyPath(key),
			password,
		)
		if err != nil {
			return fmt.Errorf("PEM file not found: %s", key)
		}
		auth = sshAuth
	case strings.HasPrefix(url, "http"):
		auth = &http.BasicAuth{
			Username: user,
			Password: password,
		}

	default:
		return fmt.Errorf("unsupported remote URL: %s", url)
	}

	// push current tags and commits
	return r.git.Push(&git.PushOptions{
		RefSpecs: []config.RefSpec{
			config.RefSpec("+refs/heads/*:refs/heads/*"),
			config.RefSpec("+refs/tags/*:refs/tags/*"),
		},
		Auth: auth,
	})
}

// iterates over tags finding latest semantic tag
// returns assigned semantic version and commit hash
func (r *Repository) getLatestVersion() (string, string, error) {
	var hash string
	current, _ := semver.NewVersion(initialVersion)

	tags, err := r.git.Tags()
	if err != nil {
		return "", "", err
	}

	tags.ForEach(func(p *plumbing.Reference) error {
		tag := p.Name().Short()
		version, _ := semver.NewVersion(tag)
		if current.LessThan(version) {
			current = version
			hash = p.Hash().String()
		}
		return nil
	})

	return current.String(), hash, err
}

// returns lastest commit and tree status
func (r *Repository) getTreeStatus() (string, bool, error) {
	var hash string
	var isDirty bool

	ref, err := r.git.Head()
	if err != nil {
		return hash, isDirty, fmt.Errorf("%s - is repository empty?", err.Error())
	}
	hash = ref.Hash().String()
	w, err := r.git.Worktree()
	if err != nil {
		return hash, isDirty, err
	}
	status, err := w.Status()
	if err != nil {
		return hash, isDirty, err
	}
	return hash, !status.IsClean(), nil
}

// returns next possible semantic version
func (r *Repository) getNextVersion(prev, prevHash string) (string, []Commit, error) {
	v, err := semver.NewVersion(prev)
	if err != nil {
		return "", []Commit{}, err
	}
	commits, err := r.commitsSince(prevHash)
	if err != nil {
		return "", commits, err
	}
	var bump SemanticVersion
	for _, commit := range commits {
		if commit.Version > bump {
			bump = commit.Version
		}
	}

	var vNext semver.Version
	switch bump {
	case VersionMajor:
		vNext = v.IncMajor()
	case VersionMinor:
		vNext = v.IncMinor()
	case VersionPatch:
		vNext = v.IncPatch()
	default:
		return "", commits, nil
	}
	return vNext.String(), commits, nil
}

// returns all commits since specified hash of history beginning
func (r *Repository) commitsSince(hash string) ([]Commit, error) {
	commits := []Commit{}
	ref, err := r.git.Head()
	if err != nil {
		return commits, fmt.Errorf("%s - is repository empty?", err.Error())
	}
	iterator, err := r.git.Log(&git.LogOptions{
		From:  ref.Hash(),
		Order: git.LogOrderCommitterTime,
	})
	if err != nil {
		return commits, err
	}
	err = iterator.ForEach(func(c *object.Commit) error {
		if c.Hash.String() == hash {
			return storer.ErrStop
		}
		commit, err := r.parseCommitMessage(c.Message)
		commit.Hash = c.Hash.String()
		commits = append(commits, commit)
		return err
	})
	return commits, err
}

func (r *Repository) parseCommitMessage(text string) (Commit, error) {
	c := Commit{}
	cType, cScope, cMessage := tokenizeCommitMessage(text)
	c.Type = cType
	c.Scope = cScope
	c.Message = cMessage

	switch {
	case strings.Contains(cMessage, breakingChangeText):
		c.Version = VersionMajor
	case inStringSlice(c.Type, r.typeMajor):
		c.Version = VersionMajor
	case inStringSlice(c.Type, r.typeMinor):
		c.Version = VersionMinor
	case inStringSlice(c.Type, r.typePatch):
		c.Version = VersionPatch
	default:
		c.Version = VersionNone
	}

	return c, nil
}

// finds the first subdirectory containing repository and returns it
func openRepoRecursive(repoPath string) (*git.Repository, error) {
	path, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	}
	for path != "/" {
		repo, err := git.PlainOpen(path)
		if err != git.ErrRepositoryNotExists {
			return repo, err
		}
		path = filepath.Dir(path)
	}
	return nil, git.ErrRepositoryNotExists
}

// return { type, scope, message }
func tokenizeCommitMessage(message string) (string, string, string) {
	var re *regexp.Regexp
	var result []string

	// check for "type(scope): message"
	re = regexp.MustCompile(`^(\w+)\((\w+)\): (.+)`)
	result = re.FindStringSubmatch(message)
	if len(result) > 0 {
		if result[2] == "*" {
			result[2] = ""
		}
		return result[1], result[2], result[3]
	}

	// check for "type: message"
	re = regexp.MustCompile(`^(\w+): (.+)`)
	result = re.FindStringSubmatch(message)
	if len(result) > 0 {
		return result[1], "", result[2]
	}
	return "", "", message
}

func shortHash(hash string) string {
	return hash[:7]
}

func createTag(version, commit string, isDirty bool) string {
	v, _ := semver.NewVersion(version)
	if commit != "" {
		vNext, _ := v.SetPrerelease(shortHash(commit))
		v = &vNext
	}
	if isDirty {
		vNext, _ := v.SetMetadata("dirty")
		v = &vNext
	}
	return v.String()
}

func inStringSlice(item string, arr []string) bool {
	for _, check := range arr {
		if check == item {
			return true
		}
	}
	return false
}

func sanitizeSSHKeyPath(key string) string {
	if strings.HasPrefix(key, "~") {
		home, _ := os.UserHomeDir()
		key = home + key[1:]
	}
	return key
}
