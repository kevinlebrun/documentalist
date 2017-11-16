package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"sync"

	gitlabAPI "github.com/xanzy/go-gitlab"
	"gopkg.in/go-playground/webhooks.v2"
	"gopkg.in/go-playground/webhooks.v2/gitlab"
)

const (
	AssetsDir string = "assets"
)

var AssetsBaseURL *string

var (
	Title    string = "Documentalist"
	SubTitle string = "Here lies your documentations"
)

func assertExecutable(command string) {
	_, err := exec.LookPath(command)
	if err != nil {
		panic(fmt.Errorf("%s is required to continue", command))
	}
}

func main() {
	var (
		gitlabAccessToken = flag.String("gitlab-access-token", "", "A valid gitlab access token.")
		gitlabBaseURL     = flag.String("gitlab-base-url", "https://gitlab.com/api/v3/", "The gilab base URL.")
		assetsPort        = flag.Int("assets-port", 8182, "The port on which assets will be served.")
		hooksPort         = flag.Int("hooks-port", 8181, "The port on which webooks will be served.")
		secret            = flag.String("secret", "", "Secret used to protect webhook server.")
	)

	AssetsBaseURL = flag.String("assets-base-url", "", "The assets base URL used for permalink generation.")

	flag.Parse()

	assertExecutable("git")

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}

	// Disable HTTP2, it doesn't work well with our nginx version
	http.DefaultClient.Transport = &http.Transport{
		TLSNextProto: make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
	}

	gl := gitlabAPI.NewClient(nil, *gitlabAccessToken)
	gl.SetBaseURL(*gitlabBaseURL)

	in := make(chan DocumentationRequest, 10)
	messages := make(chan MergeRequestMessageOptions, 10)

	hook := gitlab.New(&gitlab.Config{Secret: *secret})
	hook.RegisterEvents(HandlePush(in), gitlab.PushEvents)
	hook.RegisterEvents(HandleMergeRequest(in), gitlab.MergerRequestEvents)

	go DocumentGenerator(path.Join(path.Dir(ex), path.Join(AssetsDir, "projects")), in, messages)
	go Notifier(gl, *AssetsBaseURL+":"+strconv.Itoa(*assetsPort), messages)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		log.Printf("webhook server listening on port %d\n", *hooksPort)
		err = webhooks.Run(hook, ":"+strconv.Itoa(*hooksPort), "/webhooks")
		if err != nil {
			log.Fatal(err)
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		http.HandleFunc("/", home(gl))
		log.Printf("assets server listening on port %d\n", *assetsPort)
		err := http.ListenAndServe(":"+strconv.Itoa(*assetsPort), nil)
		if err != nil {
			log.Fatal(err)
		}
		wg.Done()
	}()

	wg.Wait()
}

type ProjectEntry struct {
	Name          string
	Description   string
	GitlabLink    string
	Refs          []*RefEntry
	MergeRequests []*MergeRequestEntry
}

type RefEntry struct {
	Name              string
	GitlabLink        string
	DocumentalistLink string
}

type MergeRequestEntry struct {
	Name              string
	GitlabLink        string
	DocumentalistLink string
}

type IndexPage struct {
	ProjectEntries []*ProjectEntry
	Title          string
	SubTitle       string
}

func home(client *gitlabAPI.Client) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			fs := http.FileServer(http.Dir(AssetsDir))
			fs.ServeHTTP(w, r)
			return
		}

		type Project struct {
			ID            int
			Refs          []string
			MergeRequests []int
		}

		var projects []Project

		ps, _ := ioutil.ReadDir("assets/projects")

		for _, p := range ps {
			rs, _ := ioutil.ReadDir(path.Join("assets/projects", p.Name(), "refs"))

			refs := make([]string, 0, len(rs))
			for _, r := range rs {
				refs = append(refs, r.Name())
			}

			ms, _ := ioutil.ReadDir(path.Join("assets/projects", p.Name(), "merge_requests"))

			mrs := make([]int, 0, len(ms))
			for _, m := range ms {
				i, _ := strconv.Atoi(m.Name())
				mrs = append(mrs, i)
			}

			id, _ := strconv.Atoi(p.Name())
			projects = append(projects, Project{id, refs, mrs})
		}

		var projectEntries []*ProjectEntry

		for _, project := range projects {
			// TODO think about caching
			p, _, _ := client.Projects.GetProject(project.ID)

			projectEntry := &ProjectEntry{
				Name:        p.Name,
				Description: p.Description,
				GitlabLink:  p.WebURL,
			}

			bs, _, _ := client.Branches.ListBranches(project.ID, nil)

			for _, bName := range project.Refs {
				var branch *gitlabAPI.Branch
				for _, b := range bs {
					if b.Name == bName {
						branch = b
						break
					}
				}

				if branch == nil {
					continue
				}

				projectEntry.Refs = append(projectEntry.Refs, &RefEntry{
					Name:              branch.Name,
					GitlabLink:        projectEntry.GitlabLink + "/tree/" + branch.Name,
					DocumentalistLink: *AssetsBaseURL + "/projects/" + strconv.Itoa(project.ID) + "/refs/" + branch.Name,
				})
			}

			options := &gitlabAPI.ListProjectMergeRequestsOptions{State: gitlabAPI.String("opened")}
			mrs, _, _ := client.MergeRequests.ListProjectMergeRequests(project.ID, options)

			for _, mrID := range project.MergeRequests {
				var mr *gitlabAPI.MergeRequest
				for _, m := range mrs {
					if m.ID == mrID {
						mr = m
						break
					}
				}
				if mr == nil {
					continue
				}

				projectEntry.MergeRequests = append(projectEntry.MergeRequests, &MergeRequestEntry{
					Name:              mr.Title,
					GitlabLink:        projectEntry.GitlabLink + "/merge_requests/" + strconv.Itoa(mr.IID),
					DocumentalistLink: *AssetsBaseURL + "/projects/" + strconv.Itoa(project.ID) + "/merge_requests/" + strconv.Itoa(mr.ID),
				})
			}

			projectEntries = append(projectEntries, projectEntry)
		}

		t, _ := template.ParseFiles("index.html")
		t.Execute(w, &IndexPage{
			ProjectEntries: projectEntries,
			Title:          Title,
			SubTitle:       SubTitle,
		})
	}
}
