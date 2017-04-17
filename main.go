package main

import (
	"crypto/tls"
	"flag"
	"fmt"
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
		assetsBaseURL     = flag.String("assets-base-url", "", "The assets base URL used for permalink generation.")
		assetsPort        = flag.Int("assets-port", 8182, "The port on which assets will be served.")
		hooksPort         = flag.Int("hooks-port", 8181, "The port on which webooks will be served.")
		secret            = flag.String("secret", "", "Secret used to protect webhook server.")
	)

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

	go DocumentGenerator(path.Join(path.Dir(ex), AssetsDir), in, messages)
	go Notifier(gl, *assetsBaseURL+":"+strconv.Itoa(*assetsPort), messages)

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
		fs := http.FileServer(http.Dir(AssetsDir))
		http.Handle("/", fs)
		log.Printf("assets server listening on port %d\n", *assetsPort)
		err := http.ListenAndServe(":"+strconv.Itoa(*assetsPort), nil)
		if err != nil {
			log.Fatal(err)
		}
		wg.Done()
	}()

	wg.Wait()
}
