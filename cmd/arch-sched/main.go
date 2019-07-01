package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	library "github.com/sylabs/scs-library-client/client"
	v1 "k8s.io/api/core/v1"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/api"
)

const (
	defaultPort       = "8888"
	sylabsImagePrefix = "cloud.sylabs.io"
)

var version = "unknown"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(version)
		return
	}

	// todo support caching?

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}
	http.HandleFunc("/filter", handleFilter)
	log.Println("Starting extender...")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func handleFilter(w http.ResponseWriter, req *http.Request) {
	log.Println("Got filter request!")

	w.Header().Set("Content-Type", "application/json")
	resp := json.NewEncoder(w)

	var extenderArgs schedulerapi.ExtenderArgs
	err := json.NewDecoder(req.Body).Decode(&extenderArgs)
	if err != nil {
		log.Printf("could not decode filter request: %v", err)
		_ = resp.Encode(schedulerapi.ExtenderFilterResult{
			Error: err.Error(),
		})
		return
	}
	log.Printf("Request body: %v", extenderArgs)

	extenderFilterResult, err := filter(req.Context(), extenderArgs)
	if err != nil {
		log.Printf("could not filter nodes: %v", err)
		_ = resp.Encode(schedulerapi.ExtenderFilterResult{
			Error: err.Error(),
		})
		return
	}

	err = resp.Encode(extenderFilterResult)
	if err != nil {
		log.Printf("could not encode filter response: %v", err)
		_ = resp.Encode(schedulerapi.ExtenderFilterResult{
			Error: err.Error(),
		})
		return
	}
}

// filter filters nodes according to predicates defined in this extender
// it's webhooked to pkg/scheduler/core/generic_scheduler.go#findNodesThatFit()
func filter(ctx context.Context, args schedulerapi.ExtenderArgs) (*schedulerapi.ExtenderFilterResult, error) {
	pod := args.Pod

	// support a single container pod for now
	if len(pod.Spec.Containers) != 1 {
		return &schedulerapi.ExtenderFilterResult{
			Nodes: args.Nodes,
		}, nil
	}

	cont := pod.Spec.Containers[0]

	// support SIF images for now
	if !strings.HasPrefix(cont.Image, sylabsImagePrefix) {
		return &schedulerapi.ExtenderFilterResult{
			Nodes: args.Nodes,
		}, nil
	}

	config := &library.Config{}
	client, err := library.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("could not create library client: %v", err)
	}

	image := strings.TrimPrefix(cont.Image, sylabsImagePrefix)
	img, found, err := client.GetImage(ctx, image)
	if err != nil {
		return nil, fmt.Errorf("could not get library image info: %v", err)
	}
	if !found {
		return nil, fmt.Errorf("library image %q is not found", cont.Image)
	}

	// can't say anything in this case
	if img.Architecture == nil {
		return &schedulerapi.ExtenderFilterResult{
			Nodes: args.Nodes,
		}, nil
	}

	var filteredNodes []v1.Node
	failedNodes := make(schedulerapi.FailedNodesMap)

	// todo parallelize this?
	imgArch := *img.Architecture
	for _, node := range args.Nodes.Items {

		nodeArch := node.Status.NodeInfo.Architecture
		if imgArch == nodeArch {
			filteredNodes = append(filteredNodes, node)
		} else {
			failedNodes[node.Name] = fmt.Sprintf("image arch %q doesn't fit node arch %q", imgArch, nodeArch)
		}
	}

	return &schedulerapi.ExtenderFilterResult{
		Nodes: &v1.NodeList{
			Items: filteredNodes,
		},
		FailedNodes: failedNodes,
	}, nil
}
