package main

// To Run: ./libpod/bin/restapi
// To Test: curl {ip}:8080/images # GET Images
// To Test: curl {ip}:8080/images | json_reformat # GET Images Pretty Print
// To Test: curl {ip}:8080/image?id=599aae32efb4 # GET Image by ID
// To Test: curl {ip}:8080/image?id=599aae32efb4 # DELETE Image by ID
// To Test: curl {ip}:8080/containers # GET Containers
// To Test: curl {ip}:8080/container?id=389cb2778c16 # GET Container by ID
// To Test: curl {ip}:8080/container?id=389cb2778c16 # DELETE Container by ID
// To Test: curl {ip}:8080/image/pull/{name} # POST pull image

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/containers/image/types"
	"github.com/containers/storage/pkg/reexec"
	"github.com/gorilla/mux"
	digest "github.com/opencontainers/go-digest"
        "github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
        "github.com/cri-o/ocicni/pkg/ocicni"
        "k8s.io/apimachinery/pkg/fields"

)

var runtime *libpod.Runtime

//var runtime *image.Runtime

// Possible Util definition

type imagesJSONParams struct {
	ID      string        `json:"id"`
	ShortID string        `json:"shortid"`
	Name    []string      `json:"names"`
	Digest  digest.Digest `json:"digest"`
	Created time.Time     `json:"created"`
	Size    *uint64       `json:"size"`
}

// containersJSONParams is used as a base structure for the psParams
// If template output is requested, containersJSONParam will be converted to
// containersJSONParams.
// containersJSONParams will be populated by data from libpod.Container,
// the members of the struct are the sama data types as their sources.
type containersJSONParams struct {
        ID               string                `json:"id"`
	ShortID          string                `json:"shortid"`
        Image            string                `json:"image"`
        ImageID          string                `json:"image_id"`
        Command          []string              `json:"command"`
        ExitCode         int32                 `json:"exitCode"`
        Exited           bool                  `json:"exited"`
        CreatedAt        time.Time             `json:"createdAt"`
        StartedAt        time.Time             `json:"startedAt"`
        ExitedAt         time.Time             `json:"exitedAt"`
        Status           string                `json:"status"`
        PID              int                   `json:"PID"`
        Ports            []ocicni.PortMapping  `json:"ports"`
        Size             *shared.ContainerSize `json:"size,omitempty"`
        Names            string                `json:"names"`
        Labels           fields.Set            `json:"labels"`
        Mounts           []string              `json:"mounts"`
        ContainerRunning bool                  `json:"ctrRunning"`
        Namespaces       *shared.Namespace     `json:"namespace,omitempty"`
        Pod              string                `json:"pod,omitempty"`
        IsInfra          bool                  `json:"infra"`
}

// End Possible Util definition

func main() {
	if reexec.Init() {
		return
	}

	if runtime == nil {
		getRuntime()
	}
	fmt.Println("LibPod Rest API Starting")
	router := mux.NewRouter()
	router.HandleFunc("/containers", GetContainers).Methods("GET")
	router.HandleFunc("/container", GetContainer).Methods("GET")
	router.HandleFunc("/container", DeleteContainer).Methods("DELETE")
	router.HandleFunc("/images", GetImages).Methods("GET")
	router.HandleFunc("/image", GetImage).Methods("GET")
	router.HandleFunc("/image", DeleteImage).Methods("DELETE")
	router.HandleFunc("/image/pull/{name}", PullImage).Methods("POST")
	log.Fatal(http.ListenAndServe(":8080", router))
}


func DeleteImage(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type","application/json")
	input := r.URL.Query().Get("id")
	image, err := runtime.ImageRuntime().NewFromLocal(input)
	if err != nil {
		result := fmt.Sprintf("unable to find image, %v", err)
		json.NewEncoder(w).Encode(result)
		return
	}

	ctx := context.TODO()
	// TODO: false for boolean force, fix later.
	imageID, deleteErr := runtime.RemoveImage(ctx, image, false)
	if deleteErr != nil {
		errString := fmt.Sprintf("unable to delete image %v, error: %v", image.ID(), deleteErr)
		json.NewEncoder(w).Encode(errString)
	} else {
		msg := fmt.Sprintf("Deleted Image ID: %s", imageID)
		json.NewEncoder(w).Encode(msg)
	}
}

func GetImage(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type","application/json")
	var images[]*image.Image
	input := r.URL.Query().Get("id")
	image, err := runtime.ImageRuntime().NewFromLocal(input)
	if err != nil {
		result := fmt.Sprintf("unable to get image, %v", err)
		json.NewEncoder(w).Encode(result)
		return
	}

	ctx := context.TODO()
	images = append(images, image)
	imagesOutput := getImagesJSONOutput(ctx, runtime, images)
	json.NewEncoder(w).Encode(imagesOutput)
}


func GetImages(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type","application/json")
	images, err := runtime.ImageRuntime().GetImages()
	if err != nil {
		result := fmt.Sprintf("unable to get images, %v", err)
		json.NewEncoder(w).Encode(result)
		return
	}
	ctx := context.TODO()
	if len(images) == 0 {
                result := fmt.Sprintf("no images found, %v", err)
                json.NewEncoder(w).Encode(result)
		return
	}

	imagesOutput := getImagesJSONOutput(ctx, runtime, images)
	json.NewEncoder(w).Encode(&imagesOutput)
}


func DeleteContainer(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type","application/json")
	input := r.URL.Query().Get("id")
	container, err := runtime.LookupContainer(input)
	if err != nil {
		result := fmt.Sprintf("unable to get container, %v", err)
		json.NewEncoder(w).Encode(result)
		return
	}

	ctx := context.TODO()
	// TODO: false for boolean force, fix later.
	deleteErr := runtime.RemoveContainer(ctx, container, false)
	if deleteErr != nil {
		errString := fmt.Sprintf("unable to delete container %v, error: %v", container.ID(), deleteErr)
		json.NewEncoder(w).Encode(errString)
	} else {
		msg := fmt.Sprintf("Deleted Container ID: %s", container.ID())
		json.NewEncoder(w).Encode(msg)
	}
}


func GetContainer(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type","application/json")
	var containers[]*libpod.Container
	input := r.URL.Query().Get("id")
	container, err := runtime.LookupContainer(input)
	if err != nil {
		result := fmt.Sprintf("unable to get container, %v", err)
		json.NewEncoder(w).Encode(result)
		return
	}

	ctx := context.TODO()
	containers = append(containers, container)
        containersOutput := getContainersJSONOutput(ctx, runtime, containers)
        json.NewEncoder(w).Encode(&containersOutput)
}


func GetContainers(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type","application/json")
        containers, err := runtime.GetContainers()
        if err != nil {
                result := fmt.Sprintf("unable to get containers, %v", err)
                json.NewEncoder(w).Encode(result)
                return
        }
        ctx := context.TODO()
        if len(containers) == 0 {
                result := fmt.Sprintf("no containers found, %v", err)
                json.NewEncoder(w).Encode(result)
                return
        }

        containersOutput := getContainersJSONOutput(ctx, runtime, containers)
        json.NewEncoder(w).Encode(&containersOutput)
}

// pull the specified image
func PullImage(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

        var registryCreds *types.DockerAuthConfig
        var images[]*image.Image

        dockerRegistryOptions := image.DockerRegistryOptions{
                DockerRegistryCreds: registryCreds,
                DockerCertPath:      "",
        }

	ctx := context.TODO()
	newImage, err := runtime.ImageRuntime().New(ctx, params["name"], "", "", nil, &dockerRegistryOptions, image.SigningOptions{}, true)
        if err != nil {
                result := fmt.Sprintf("image not found, %v", err)
                json.NewEncoder(w).Encode(result)
        }

        images = append(images, newImage)
        imagesOutput := getImagesJSONOutput(ctx, runtime, images)
        json.NewEncoder(w).Encode(imagesOutput)

}


// Possible util code:

// getContainersJSONOutput returns the images information in its raw form
func getContainersJSONOutput(ctx context.Context, runtime *libpod.Runtime, containers []*libpod.Container) (containersOutput []containersJSONParams) {

        for _, ctr := range containers {
        	_, imageName := ctr.Image()
                exitCode, exited, _ := ctr.ExitCode()
                startedAt, _ := ctr.StartedTime()
                exitedAt, _ := ctr.FinishedTime()
		status, _ := ctr.State()
		imageID := ""
	        image, err := runtime.ImageRuntime().NewFromLocal(imageName)
		if err == nil {
			imageID = image.ID()
		}
		labels := ctr.Labels()
                params := containersJSONParams{
                        ID:      ctr.ID(),
			ShortID: ctr.ID()[0:12],
                        Names:   ctr.Name(),
                        Image:   imageName,
			ImageID: imageID,
			Command: ctr.Command(),
			ExitCode: exitCode,
			Exited:   exited,
                        CreatedAt: ctr.CreatedTime(),
			StartedAt: startedAt,
                        ExitedAt: exitedAt,
			Status:   status.String(),
			Labels:   labels,			
			IsInfra:  ctr.IsInfra(),
                }
                containersOutput = append(containersOutput, params)
        }
        return
}


// getImagesJSONOutput returns the images information in its raw form
func getImagesJSONOutput(ctx context.Context, runtime *libpod.Runtime, images []*image.Image) (imagesOutput []imagesJSONParams) {
	for _, img := range images {
		size, err := img.Size(ctx)
		if err != nil {
			size = nil
		}
		params := imagesJSONParams{
			ID:      img.ID(),
			ShortID: img.ID()[0:12],
			Name:    img.Names(),
			Digest:  img.Digest(),
			Created: img.Created(),
			Size:    size,
		}
		imagesOutput = append(imagesOutput, params)
	}
	return
}

// End Possible util code

func getRuntime() {
	var err error
	options := []libpod.RuntimeOption{}
	//	storageOpts := storage.DefaultStoreOptions
	//	options = append(options, libpod.WithStorageConfig(storageOpts))
	runtime, err = libpod.NewRuntime(options...)

	// An empty StoreOptions will use defaults, which is /var/lib/containers/storage
	// If you define GraphRoot and RunRoot to a tempdir, it will create a new storage
	//	storeOpts := storage.StoreOptions{}
	// The first step to using the image library is almost always to create an
	// imageRuntime.
	//	runtime, err = image.NewImageRuntimeFromOptions(storeOpts)

	fmt.Printf("Runtime %v\n", runtime)
	if err != nil {
		result := fmt.Sprintf("unable to get libpod runtime %v\n", err)
		fmt.Printf(result)
		os.Exit(1)
	}
	return
}
