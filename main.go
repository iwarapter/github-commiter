package main

import (
	"context"
	"encoding/base64"
	"github.com/go-git/go-git/v5"
	"github.com/jessevdk/go-flags"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
	"log"
	"os"
)

func main() {
	var opts struct {
		Repository string `short:"r" long:"repository" description:"the repository to push commits to" required:"true"`
		BranchName string `short:"b" long:"branch" description:"the branch to push commits to" required:"true"`
		Message    string `short:"m" long:"message" description:"the commit message to use" default:"updated with github-signer"`
	}
	_, err := flags.Parse(&opts)
	switch e := err.(type) {
	case *flags.Error:
		if e.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	case nil:
		break
	default:
		log.Fatal(err)
	}

	r, err := git.PlainOpen(".")
	if err != nil {
		log.Fatalf("unable to open repository: %s", err)
	}

	w, err := r.Worktree()
	if err != nil {
		log.Fatalf("unable to open repository: %s", err)
	}
	rev, err := r.Head()
	if err != nil {
		log.Fatalf("unable to find HEAD revision: %s", err)
	}
	s, err := w.Status()
	if err != nil {
		log.Fatalf("unable to open repository: %s", err)
	}
	changes := &[]githubv4.FileAddition{}
	for name, status := range s {
		if status.Worktree == git.Modified {
			b, _ := os.ReadFile(name)
			content := base64.StdEncoding.EncodeToString(b)
			*changes = append(*changes, githubv4.FileAddition{
				Path:     githubv4.String(name),
				Contents: githubv4.Base64String(content),
			})
		}
	}
	if len(*changes) == 0 {
		log.Printf("no changes to commit, exiting")
		os.Exit(0)
	}

	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)

	client := githubv4.NewClient(httpClient)

	var m struct {
		CreateCommitOnBranch struct {
			Commit struct {
				Url githubv4.ID
			}
		} `graphql:"createCommitOnBranch(input: $input)"`
	}
	input := githubv4.CreateCommitOnBranchInput{
		Branch: githubv4.CommittableBranch{
			RepositoryNameWithOwner: githubv4.NewString(githubv4.String(opts.Repository)),
			BranchName:              githubv4.NewString(githubv4.String(opts.BranchName)),
		},
		Message: githubv4.CommitMessage{Headline: githubv4.String(opts.Message)},
		FileChanges: &githubv4.FileChanges{
			Additions: changes,
		},
		ExpectedHeadOid: githubv4.GitObjectID(rev.Hash().String()),
	}

	err = client.Mutate(context.Background(), &m, input, nil)
	if err != nil {
		log.Fatalf("unable to mutate: %s", err)
	}
	log.Printf("mutation complete: %s", m.CreateCommitOnBranch.Commit.Url)
}
