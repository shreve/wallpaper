package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const API = "https://source.unsplash.com/random/%s/?%s"
const Latest = "/tmp/latest-wallpaper.jpg"

const URISep = "%20"
const FSSep = "-"

var UserPaperDir = "/home/jacob/Pictures/wallpapers/"

func fail(err error, msg string) {
	if err != nil {
		fmt.Println(msg)
		os.Exit(1)
	}
}

func runChain(cmds []*exec.Cmd) (out bytes.Buffer) {
	writePipes := make([]*io.PipeWriter, 0)
	for i := 0; i < len(cmds)-1; i++ {
		r, w := io.Pipe()
		cmds[i].Stdout = w
		cmds[i+1].Stdin = r
		writePipes = append(writePipes, w)
	}
	cmds[len(cmds)-1].Stdout = &out

	for i := 0; i < len(cmds); i++ {
		cmds[i].Start()
	}
	for i := 0; i < len(cmds)-1; i++ {
		cmds[i].Wait()
		writePipes[i].Close()
	}
	cmds[len(cmds)-1].Wait()

	return
}

func resolution() string {
	out := runChain([]*exec.Cmd{
		exec.Command("xrandr"),
		exec.Command("grep", "*"),
		exec.Command("awk", "{print $1}"),
	})
	result := out.String()
	return result[:len(result)-1]
}

func usage() {
	fmt.Println("wall is a CLI for fetching and saving wallpaper-sized images")
	fmt.Println("from Unsplash.com. By default, it lets you try out new ones")
	fmt.Println("and save the ones you like.")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  wall command [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  set [query]\t pick a random local wallpaper")
	fmt.Println("  new [query]\t fetch a random image with an optional query")
	fmt.Println("  save\t\t save the last wallpaper to a permanent location")
	fmt.Println("")
	os.Exit(0)
}

func SearchAndSave(query string) string {
	url := fmt.Sprintf(API, resolution(), query)
	fmt.Println("Fetching image from", url)

	response, err := http.Get(url)
	fail(err, "There was a problem reaching unsplash.com")
	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	fail(err, "There was a problem trying to read the response from unsplash.com")

	sum := md5.Sum(data)
	var name string
	if query == "" {
		name = fmt.Sprintf("/tmp/%x.jpg", sum)
	} else {
		query = strings.ReplaceAll(query, URISep, FSSep)
		name = fmt.Sprintf("/tmp/%s-%x.jpg", query, sum)
	}
	fmt.Println(name)

	file, err := os.Create(name)
	fail(err, "There was a problem creating the file to save the download")

	_, err = file.Write(data)
	fail(err, "There was a problem saving the download")
	file.Close()

	os.Remove(Latest)
	os.Symlink(name, Latest)

	ApplyWallpaper(name)

	return name
}

func ApplyWallpaper(name string) {
	exec.Command("feh", "--bg-fill", name).Run()
}

func copyFile(src, dst string) {
	in, err := os.Open(src)
	if err != nil {
		panic(err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		panic(err)
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		panic(err)
	}

	source, err := os.Stat(src)
	if err != nil {
		panic(err)
	}
	err = os.Chmod(dst, source.Mode())
	if err != nil {
		panic(err)
	}
}

func CopyLatestInPlace() {
	latestName, err := os.Readlink(Latest)
	if err != nil {
		return
	}
	filename := filepath.Base(latestName)
	dest := UserPaperDir + filename
	fmt.Println("Moving", latestName, "to", dest)
	copyFile(latestName, dest)
	ApplyWallpaper(dest)
}

func SetFromLocal(query string) {
	bits := strings.Split(query, " ")
	files := make([]string, 0)
	filepath.Walk(UserPaperDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		for _, bit := range bits {
			if !strings.Contains(path, bit) {
				return nil
			}
		}
		files = append(files, path)
		return nil
	})

	rand.Seed(time.Now().Unix())
	i := rand.Intn(len(files))
	ApplyWallpaper(files[i])
}

func main() {
	if len(os.Args) == 1 {
		usage()
		return
	}

	query := ""
	if len(os.Args) >= 3 {
		query = strings.Join(os.Args[2:], " ")
	}

	switch os.Args[1] {
	// case "gallery":
	// 	new, but on a loop with a brief delay
	case "set":
		SetFromLocal(query)
	case "new":
		query = strings.ReplaceAll(query, " ", URISep)
		SearchAndSave(query)
	case "save":
		CopyLatestInPlace()
	default:
		usage()
	}
}
