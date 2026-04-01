package serve

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fobilow/ham"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
)

const reloadScript = `<script>(function(){var s=new EventSource("/__ham/events");s.onmessage=function(e){if(e.data==="reload")location.reload();};})()</script>`

type devServer struct {
	workingDir string
	port       string
	site       *ham.Site
	clients    map[chan string]struct{}
	mu         sync.Mutex
}

func Run(workingDir, port string) {
	s := &devServer{
		workingDir: workingDir,
		port:       port,
		site:       ham.NewSite(),
		clients:    make(map[chan string]struct{}),
	}

	// initial build
	log.Println("Running initial build...")
	if err := s.build(); err != nil {
		log.Fatal("initial build failed: ", err)
	}

	// start file watcher
	go s.watch()

	// start HTTP server
	router := gin.Default()
	router.GET("/__ham/events", s.handleSSE)
	router.NoRoute(s.handleRequest)

	srv := &http.Server{Addr: ":" + s.port, Handler: router}

	// clean up output directory on shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Println("Shutting down, cleaning up output directory...")
		outputDir := filepath.Join(s.workingDir, ham.DefaultOutputDir)
		if err := os.RemoveAll(outputDir); err != nil {
			log.Println("Failed to clean output directory:", err)
		}

		srv.Shutdown(context.Background())
	}()

	fmt.Printf("\n  HAM dev server running at http://localhost:%s\n\n", s.port)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func (s *devServer) build() error {
	return s.site.Build(s.workingDir, ham.DefaultOutputDir)
}

func (s *devServer) broadcast(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (s *devServer) addClient() chan string {
	ch := make(chan string, 1)
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}

func (s *devServer) removeClient(ch chan string) {
	s.mu.Lock()
	delete(s.clients, ch)
	s.mu.Unlock()
	close(ch)
}

func (s *devServer) handleSSE(c *gin.Context) {
	ch := s.addClient()
	defer s.removeClient(ch)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Flush()

	ctx := c.Request.Context()
	for {
		select {
		case msg := <-ch:
			fmt.Fprintf(c.Writer, "data: %s\n\n", msg)
			c.Writer.Flush()
		case <-ctx.Done():
			return
		}
	}
}

func (s *devServer) handleRequest(c *gin.Context) {
	uri := strings.Split(c.Request.RequestURI, "?")[0]
	dir, file := filepath.Split(uri)
	if file == "" {
		file = "index.html"
	}

	filePath := filepath.Join(s.workingDir, ham.DefaultOutputDir, dir, file)

	if filepath.Ext(file) == ".html" {
		b, err := os.ReadFile(filePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.AbortWithStatus(http.StatusNotFound)
			} else {
				c.AbortWithStatus(http.StatusInternalServerError)
			}
			return
		}
		// inject reload script before </body>
		modified := bytes.Replace(b, []byte("</body>"), []byte(reloadScript+"</body>"), 1)
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Data(http.StatusOK, "text/html; charset=utf-8", modified)
		return
	}

	c.File(filePath)
}

func (s *devServer) watch() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("failed to create watcher: ", err)
	}
	defer watcher.Close()

	srcDir := filepath.Join(s.workingDir, "src")

	// walk src dir and add all directories
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		log.Fatal("failed to watch src directory: ", err)
	}

	log.Println("Watching for changes in", srcDir)

	var debounce *time.Timer
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				if debounce != nil {
					debounce.Stop()
				}
				debounce = time.AfterFunc(100*time.Millisecond, func() {
					log.Println("File changed:", event.Name)
					log.Println("Rebuilding...")

					// watch newly created directories
					if event.Has(fsnotify.Create) {
						if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
							watcher.Add(event.Name)
						}
					}

					if err := s.build(); err != nil {
						log.Println("Build error:", err)
						return
					}
					log.Println("Build complete, reloading browsers...")
					s.broadcast("reload")
				})
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("Watcher error:", err)
		}
	}
}
