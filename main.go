package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

var (
	cmd          *exec.Cmd
	state        sync.Mutex
	eventTime    = make(map[string]int64)
	scheduleTime time.Time
)

var started = make(chan bool)

func main() {
	crupath, _ := os.Getwd()
	// crupath = path.Join(crupath, "./")

	var paths []string
	readProjectDirectories(crupath, &paths)
	newWatcher(paths)

	autobuild()

	for {
		select {
		case <-started:
			log.Println("start")
		}
	}
}

func readProjectDirectories(directory string, paths *[]string) {
	fileInfos, err := ioutil.ReadDir(directory)
	if err != nil {
		return
	}

	useDirectory := false
	for _, fileInfo := range fileInfos {
		if strings.HasSuffix(fileInfo.Name(), "docs") {
			continue
		}

		if fileInfo.IsDir() == true && fileInfo.Name()[0] != '.' {
			readProjectDirectories(directory+"/"+fileInfo.Name(), paths)
			continue
		}

		if useDirectory == true {
			continue
		}

		if path.Ext(fileInfo.Name()) == ".go" {
			*paths = append(*paths, directory)
			useDirectory = true
		}
	}

	return
}

func newWatcher(paths []string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("[ERRO] Fail to create new Watcher[ %s ]\n", err)
		os.Exit(2)
	}

	go func() {
		for {
			select {
			case e := <-watcher.Events:

				log.Printf("[EVEN] get event[ %v ]\n", e)

				isbuild := true

				if !checkIfWatchExt(e.Name) {
					continue
				}

				mt := getFileModTime(e.Name)
				if t := eventTime[e.Name]; mt == t {
					isbuild = false
				}

				eventTime[e.Name] = mt

				if isbuild {
					log.Printf("[EVEN] %s\n", e)
					go func() {
						// Wait 1s before autobuild util there is no file change.
						scheduleTime = time.Now().Add(1 * time.Second)
						for {
							time.Sleep(scheduleTime.Sub(time.Now()))
							if time.Now().After(scheduleTime) {
								break
							}
							return
						}

						autobuild()
					}()
				}
			case err := <-watcher.Errors:
				log.Printf("[WARN] %s\n", err.Error())
			}
		}
	}()

	log.Printf("[INFO] Initializing watcher...\n")
	for _, path := range paths {
		log.Printf("[TRAC] Directory( %s )\n", path)
		err = watcher.Add(path)
		if err != nil {
			log.Printf("[ERRO] Fail to watch directory[ %s ]\n", err)
			os.Exit(2)
		}
	}

}

func autobuild() {
	state.Lock()
	defer state.Unlock()

	log.Printf("[INFO] start building...\n")
	path, _ := os.Getwd()
	os.Chdir(path)

	cmdName := "go"
	args := []string{"build", "-o", "main"}
	bcmd := exec.Command(cmdName, args...)
	bcmd.Stdout = os.Stdout
	bcmd.Stderr = os.Stderr

	err := bcmd.Run()
	if err != nil {
		log.Printf("[ERRO] ============== Build failed ===================\n")
		// log.Printf("Err %s", err.Error())
		return
	}
	log.Printf("[SUCC] Build was successful\n")

	restart()
}

func kill() {
	defer func() {
		if e := recover(); e != nil {
			log.Println("kill.recover -> ", e)
		}
	}()
	if cmd != nil && cmd.Process != nil {
		err := cmd.Process.Kill()
		if err != nil {
			log.Println("kill -> ", err)
		}
	}
}

func restart() {
	log.Println("kill running process")
	kill()
	go start()
}

func start() {
	log.Printf("[INFO] restarting ...\n")
	appname := "./main"

	cmd = exec.Command(appname)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("[INFO] %s is running...\n", appname)
	cmd.Run()
}

func getFileModTime(path string) int64 {
	path = strings.Replace(path, "\\", "/", -1)
	f, err := os.Open(path)
	if err != nil {
		return time.Now().Unix()
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return time.Now().Unix()
	}

	return fi.ModTime().Unix()
}

var watchExts = []string{".go", ".toml"}

func checkIfWatchExt(name string) bool {
	for _, s := range watchExts {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	return false
}
