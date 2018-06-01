package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

var (
	version = "0.0.2"
)

func main() {
	var port = flag.Int("port", 5000, "http port")
	var dataDir = flag.String("data", "data/", "path to user data directory")
	var appDir = flag.String("app", "app/", "path to maiden app directory")
	var docDir = flag.String("doc", "doc/", "path to matron lua docs")
	var debug = flag.Bool("debug", false, "enable debug logging")

	flag.Parse()

	// FIXME: pull in git version
	log.Printf("maiden (%s)", version)
	log.Printf("  port: %d", *port)
	log.Printf("   app: %s", *appDir)
	log.Printf("  data: %s", *dataDir)
	log.Printf("   doc: %s", *docDir)

	if *debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.GET("", func(ctx *gin.Context) {
		ctx.Redirect(http.StatusFound, "/maiden")
	})

	// expose app
	r.Static("/maiden", *appDir)

	// expose docs
	r.Static("/doc", *docDir)

	// api
	apiRoot := "/api/v1"
	api := r.Group(apiRoot)

	api.GET("", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, apiInfo{"maiden", version})
	})

	// dust api
	apiPrefix := filepath.Join(apiRoot, "dust")
	s := server{
		logger:       os.Stderr,
		apiPrefix:    apiPrefix,
		devicePath:   makeDevicePath(*dataDir),
		resourcePath: makeResourcePath(apiPrefix),
	}
	api.GET("/dust", s.rootListingHandler)
	api.GET("/dust/*name", s.listingHandler)
	api.PUT("/dust/*name", s.writeHandler)
	api.PATCH("/dust/*name", s.renameHandler)
	api.DELETE("/dust/*name", s.deleteHandler)

	r.Run(fmt.Sprintf(":%d", *port))
}

type server struct {
	logger       io.Writer
	apiPrefix    string
	devicePath   devicePathFunc
	resourcePath prefixFunc
}

func (s *server) logf(format string, args ...interface{}) {
	fmt.Fprintf(s.logger, format, args...)
}

func (s *server) rootListingHandler(ctx *gin.Context) {
	top := "" // MAINT: get rid of this
	entries, err := ioutil.ReadDir(s.devicePath(&top))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dir := handleDirRead("", &entries, s.resourcePath)
	ctx.JSON(http.StatusOK, dir)
}

func (s *server) listingHandler(ctx *gin.Context) {
	name := ctx.Param("name")
	path := s.devicePath(&name)

	s.logf("get of name: ", name)
	s.logf("device path: ", path)

	// figure out if this is a file or not
	info, err := os.Stat(path)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if info.IsDir() {
		entries, err := ioutil.ReadDir(path)
		if err != nil {
			// not sure why this would fail given that we just stat()'d the dir
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		prefix := filepath.Join(s.apiPrefix, name)
		subResourcePath := makeResourcePath(prefix)
		dir := handleDirRead(name, &entries, subResourcePath)
		ctx.JSON(http.StatusOK, dir)
		return
	}

	ctx.File(path)
}

func (s *server) writeHandler(ctx *gin.Context) {
	name := ctx.Param("name")
	path := s.devicePath(&name)

	kind, exists := ctx.GetQuery("kind")
	if exists && kind == "directory" {

		err := os.MkdirAll(path, 0755)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("created directory %s", path)})

	} else {
		// get code (file) blob
		file, err := ctx.FormFile("value")
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		s.logf("save path: %s\n", path)
		s.logf("content type: %s\n", file.Header["Content-Type"])

		// size, err := io.Copy(out, file)
		if err := ctx.SaveUploadedFile(file, path); err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("uploaded %s (%s, %d) to %s", file.Filename, file.Header, file.Size, path)})
	}
}

func (s *server) renameHandler(ctx *gin.Context) {
	// FIXME: this logic basically assumes PATCH == rename operation at the moment
	name := ctx.Param("name")
	path := s.devicePath(&name)

	// figure out if this exists or not
	_, err := os.Stat(path)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// compute new path
	newName, exists := ctx.GetPostForm("name")
	if !exists {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "missing 'name' key in form"})
		return
	}
	rename := filepath.Join(filepath.Dir(name), newName)
	renamePath := s.devicePath(&rename)

	s.logf("going to rename: %s to: %s\n", path, renamePath)

	err = os.Rename(path, renamePath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	info := patchInfo{s.resourcePath(rename)}
	ctx.JSON(http.StatusOK, info)
}

func (s *server) deleteHandler(ctx *gin.Context) {
	name := ctx.Param("name")
	path := s.devicePath(&name)

	s.logf("going to delete: ", path)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	err := os.RemoveAll(path)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("deleted %s", path)})
}

type devicePathFunc func(name *string) string

func makeDevicePath(prefix string) devicePathFunc {
	return func(name *string) string {
		return filepath.Join(prefix, *name)
	}
}

type prefixFunc func(...string) string

func makeResourcePath(prefix string) prefixFunc {
	return func(parts ...string) string {
		var escaped = []string{}
		for _, part := range parts {
			escaped = append(escaped, url.PathEscape(part))
		}
		return filepath.Join(prefix, filepath.Join(escaped...))
	}
}

type apiInfo struct {
	API     string `json:"api"`
	Version string `json:"version"`
}

type fileInfo struct {
	Name     string      `json:"name"`
	URL      string      `json:"url"`
	Children *[]fileInfo `json:"children,omitempty"`
}

type dirInfo struct {
	Path    string     `json:"path"`
	URL     string     `json:"url"`
	Entries []fileInfo `json:"entries"`
}

type errorInfo struct {
	Error string `json:"error"`
}

type patchInfo struct {
	URL string `json:"url"`
}

func handleDirRead(path string, entries *[]os.FileInfo, resourcePath prefixFunc) *dirInfo {
	var files = []fileInfo{}
	for _, entry := range *entries {
		var children *[]fileInfo
		if entry.IsDir() {
			children = &[]fileInfo{}
		}
		files = append(files, fileInfo{
			entry.Name(),
			resourcePath(entry.Name()),
			children,
		})
	}
	return &dirInfo{
		path,
		resourcePath(""),
		files,
	}
}
